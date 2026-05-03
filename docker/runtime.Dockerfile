FROM python:3.12-slim

WORKDIR /app

COPY python/runtime/requirements.txt ./requirements.txt
RUN pip install --no-cache-dir -r requirements.txt

COPY python/runtime ./python/runtime

WORKDIR /app/python/runtime
EXPOSE 50051
CMD ["python", "server.py"]
