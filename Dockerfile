FROM debian:stable-slim

# COPY source destination
COPY commission-tracker /bin/commission-tracker

CMD ["/bin/commission-tracker"]