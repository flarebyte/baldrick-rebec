FROM apache/age:release_PG16_1.6.0

# Install pgvector (and dependencies)
RUN apt-get update && \
    apt-get install -y postgresql-16-pgvector && \
    rm -rf /var/lib/apt/lists/*
