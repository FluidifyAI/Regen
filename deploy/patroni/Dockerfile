FROM postgres:15

# Install Patroni with etcd3 support.
# Uses the Debian-based postgres image so apt is available.
RUN apt-get update && apt-get install -y --no-install-recommends \
        python3 python3-pip python3-dev \
        libpq-dev gcc curl \
    && pip3 install --no-cache-dir "patroni[etcd3]" \
    && apt-get purge -y python3-dev gcc \
    && apt-get autoremove -y \
    && rm -rf /var/lib/apt/lists/*

# Patroni needs the postgres bin dir on PATH
ENV PATH="/usr/lib/postgresql/15/bin:${PATH}"

ENTRYPOINT ["patroni"]
CMD ["/etc/patroni/patroni.yml"]
