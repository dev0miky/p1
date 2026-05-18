FROM ghcr.io/kamailio/kamailio-ci:6.1.2-alpine

COPY kamailio.cfg /etc/kamailio/kamailio.cfg
COPY dispatcher.list /etc/kamailio/dispatcher.list

EXPOSE 5060/udp 5060/tcp 5061/tcp

HEALTHCHECK --interval=10s --timeout=3s --retries=3 \
  CMD kamcmd core.uptime || exit 1
