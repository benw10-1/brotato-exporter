## Build
FROM golang:1.23-bullseye AS build

WORKDIR /app

COPY ./gosrc/go.mod ./
COPY ./gosrc/go.sum ./
RUN go mod download

COPY ./gosrc ./
# include mod files for user mod gen
COPY ./mod ./mod

COPY ./default.yml ./

RUN go build -o /exporter-server ./cmd/exporter-server
RUN go build -o /mod-user-create ./cmd/mod-user-create

## Deploy
FROM golang:1.23-bullseye

RUN mkdir -p /var/brotatoexporter
RUN mkdir -p /var/log

VOLUME [ "/var/brotatoexporter", "/var/log" ]

WORKDIR /

# for user zip file gen
COPY --from=build /app/mod /var/lib/mod

COPY --from=build /app/default.yml /etc/brotatoexporter/default.yml

COPY --from=build /mod-user-create /mod-user-create
COPY --from=build /exporter-server /exporter-server

ENTRYPOINT ["/exporter-server"]