from typing import Any, Dict, List

from evals.fixture import build_vocabulary, row_to_evidence, term_embedding


PG_TABLE = "ddi_eval_evidence"
MILVUS_COLLECTION = "ddi_eval_evidence"


def postgres_dsn_from_env() -> str:
    import os

    return os.getenv("DDI_EVAL_PG_DSN", "dbname=ddi user=ddi password=ddi host=127.0.0.1 port=5432 connect_timeout=2")


def milvus_host_port_from_env() -> tuple[str, str]:
    import os

    return os.getenv("DDI_EVAL_MILVUS_HOST", "127.0.0.1"), os.getenv("DDI_EVAL_MILVUS_PORT", "19530")


def postgres_available() -> tuple[bool, str]:
    try:
        import psycopg2

        with psycopg2.connect(postgres_dsn_from_env()) as conn:
            with conn.cursor() as cur:
                cur.execute("select 1")
                cur.fetchone()
        return True, "ok"
    except Exception as exc:
        return False, str(exc)


def milvus_available() -> tuple[bool, str]:
    try:
        from pymilvus import connections, utility

        host, port = milvus_host_port_from_env()
        connections.connect(alias="ddi_eval_probe", host=host, port=port, timeout=2)
        utility.get_server_version(using="ddi_eval_probe")
        connections.disconnect("ddi_eval_probe")
        return True, "ok"
    except Exception as exc:
        return False, str(exc)


class DockerIndexRetriever:
    def __init__(self, rows: List[Dict[str, Any]], top_k: int = 5) -> None:
        self.rows = rows
        self.top_k = top_k
        self.vocabulary = build_vocabulary(rows)
        self.rows_by_id = {row["evidence_id"]: row for row in rows}
        self.collection = None

    def setup(self) -> None:
        self._setup_postgres()
        self._setup_milvus()

    def __call__(self, rag_name: str, query: str):
        if not query.strip():
            return []

        import psycopg2

        vector = term_embedding(query, self.vocabulary)
        if not any(vector):
            return []

        hits = self.collection.search(
            data=[vector],
            anns_field="embedding",
            param={"metric_type": "COSINE", "params": {"nprobe": 8}},
            limit=self.top_k,
            expr=f'rag_name == "{rag_name}"',
            output_fields=["evidence_id"],
        )[0]
        evidence_ids = [
            hit.entity.get("evidence_id")
            for hit in hits
            if self._has_term_overlap(hit.entity.get("evidence_id"), query)
        ]
        if not evidence_ids:
            return []

        placeholders = ", ".join(["%s"] * len(evidence_ids))
        with psycopg2.connect(postgres_dsn_from_env()) as conn:
            with conn.cursor() as cur:
                cur.execute(
                    f"""
                    select evidence_id, rag_name, source, uri, title, snippet, region, version, published_at
                    from {PG_TABLE}
                    where evidence_id in ({placeholders})
                    """,
                    evidence_ids,
                )
                rows = cur.fetchall()

        by_id = {
            row[0]: {
                "evidence_id": row[0],
                "rag_name": row[1],
                "source": row[2],
                "uri": row[3],
                "title": row[4],
                "snippet": row[5],
                "region": row[6],
                "version": row[7],
                "published_at": row[8],
            }
            for row in rows
        }
        return [row_to_evidence(by_id[evidence_id]) for evidence_id in evidence_ids if evidence_id in by_id]

    def _has_term_overlap(self, evidence_id: str | None, query: str) -> bool:
        if not evidence_id:
            return False
        row = self.rows_by_id.get(evidence_id)
        if not row:
            return False
        return any(term in query for term in row.get("terms", []))

    def _setup_postgres(self) -> None:
        import psycopg2

        with psycopg2.connect(postgres_dsn_from_env()) as conn:
            with conn.cursor() as cur:
                cur.execute(
                    f"""
                    create table if not exists {PG_TABLE} (
                        evidence_id text primary key,
                        rag_name text not null,
                        source text not null,
                        uri text not null,
                        title text not null,
                        snippet text not null,
                        region text not null,
                        version text not null,
                        published_at text not null
                    )
                    """
                )
                for row in self.rows:
                    cur.execute(
                        f"""
                        insert into {PG_TABLE}
                            (evidence_id, rag_name, source, uri, title, snippet, region, version, published_at)
                        values (%s, %s, %s, %s, %s, %s, %s, %s, %s)
                        on conflict (evidence_id) do update set
                            rag_name = excluded.rag_name,
                            source = excluded.source,
                            uri = excluded.uri,
                            title = excluded.title,
                            snippet = excluded.snippet,
                            region = excluded.region,
                            version = excluded.version,
                            published_at = excluded.published_at
                        """,
                        (
                            row["evidence_id"],
                            row["rag_name"],
                            row["source"],
                            row["uri"],
                            row["title"],
                            row["snippet"],
                            row["region"],
                            row["version"],
                            row["published_at"],
                        ),
                    )
            conn.commit()

    def _setup_milvus(self) -> None:
        from pymilvus import Collection, CollectionSchema, DataType, FieldSchema, connections, utility

        host, port = milvus_host_port_from_env()
        connections.connect(alias="default", host=host, port=port, timeout=5)

        if utility.has_collection(MILVUS_COLLECTION):
            utility.drop_collection(MILVUS_COLLECTION)

        schema = CollectionSchema(
            fields=[
                FieldSchema(name="evidence_id", dtype=DataType.VARCHAR, is_primary=True, max_length=256),
                FieldSchema(name="rag_name", dtype=DataType.VARCHAR, max_length=32),
                FieldSchema(name="embedding", dtype=DataType.FLOAT_VECTOR, dim=len(self.vocabulary)),
            ],
            description="DDI agent eval fixture index",
        )
        collection = Collection(MILVUS_COLLECTION, schema)
        collection.insert(
            [
                [row["evidence_id"] for row in self.rows],
                [row["rag_name"] for row in self.rows],
                [term_embedding(" ".join(row.get("terms", [])), self.vocabulary) for row in self.rows],
            ]
        )
        collection.flush()
        collection.create_index(
            field_name="embedding",
            index_params={"index_type": "IVF_FLAT", "metric_type": "COSINE", "params": {"nlist": 16}},
        )
        collection.load()
        self.collection = collection
