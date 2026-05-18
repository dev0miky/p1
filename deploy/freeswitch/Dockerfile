FROM safarov/freeswitch:1.10.12

COPY vars.xml /etc/freeswitch/vars.xml
COPY autoload_configs/ /etc/freeswitch/autoload_configs/
COPY sip_profiles/ /etc/freeswitch/sip_profiles/
COPY dialplan/ /etc/freeswitch/dialplan/
COPY scripts/ /usr/share/freeswitch/scripts/

EXPOSE 5060/udp 5060/tcp 5080/udp 5080/tcp 5061/tcp 7443/tcp 8021/tcp
EXPOSE 16384-32768/udp

HEALTHCHECK --interval=10s --timeout=5s --retries=3 \
  CMD fs_cli -x "status" | grep -q "UP" || exit 1
