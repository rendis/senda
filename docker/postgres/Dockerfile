FROM postgres:16-bookworm

RUN apt-get update -o Acquire::AllowInsecureRepositories=true -o Acquire::AllowDowngradeToInsecureRepositories=true \
    && apt-get install -y --no-install-recommends --allow-unauthenticated postgresql-16-cron \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

COPY init-pg-cron.sql /docker-entrypoint-initdb.d/10-pg-cron.sql
