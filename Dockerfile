FROM gcr.io/distroless/static-debian11:nonroot
ENTRYPOINT ["/baton-twingate"]
COPY baton-twingate /