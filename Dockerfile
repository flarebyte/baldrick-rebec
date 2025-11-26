FROM postgres:18.0

# Install pgvector (and dependencies)
RUN apt-get update && \
    apt-get install -y postgresql-18-pgvector && \
    rm -rf /var/lib/apt/lists/*
