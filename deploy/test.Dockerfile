FROM ubuntu:22.04

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates curl && \
    install -m 0755 -d /etc/apt/keyrings && \
    curl -fsSL https://dl.mobilint.com/apt/gpg.pub -o /etc/apt/keyrings/mblt.asc && \
    chmod a+r /etc/apt/keyrings/mblt.asc && \
    printf "%s\n" \
      "deb [signed-by=/etc/apt/keyrings/mblt.asc] https://dl.mobilint.com/apt stable multiverse" \
      > /etc/apt/sources.list.d/mobilint.list && \
    apt-get update && \
    apt-get install -y --no-install-recommends mobilint-qb-runtime mobilint-cli && \
    rm -rf /var/lib/apt/lists/*

CMD ["bash", "-lc", "\
echo MOBILINT_VISIBLE_DEVICES=${MOBILINT_VISIBLE_DEVICES}; \
for dev in ${MOBILINT_VISIBLE_DEVICES//,/ }; do \
  ls -l /dev/${dev}; \
done; \
dpkg-query -W mobilint-qb-runtime mobilint-cli; \
mobilint-cli status -L; \
sleep infinity\
"]
