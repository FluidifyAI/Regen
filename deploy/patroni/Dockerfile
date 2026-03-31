FROM postgres:15

# Install Patroni with etcd3 support inside a venv.
# python3-venv avoids PEP 668 (Debian trixie blocks system-wide pip installs).
# psycopg2-binary is a separate dep not pulled in by patroni[etcd3].
RUN apt-get update && apt-get install -y --no-install-recommends \
        python3 python3-venv curl \
    && python3 -m venv /opt/patroni \
    && /opt/patroni/bin/pip install --no-cache-dir "patroni[etcd3]" psycopg2-binary \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

ENV PATH="/opt/patroni/bin:/usr/lib/postgresql/15/bin:${PATH}"

# postgres:15 defines a 'postgres' user (uid=999). Patroni must not run as root
# because initdb refuses to execute with root privileges.
USER postgres

ENTRYPOINT ["patroni"]
CMD ["/etc/patroni/patroni.yml"]
