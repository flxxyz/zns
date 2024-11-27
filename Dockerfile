FROM public.ecr.aws/docker/library/golang:1.22-alpine3.18 as builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o zns -trimpath -ldflags "-s -w" -v .

FROM public.ecr.aws/docker/library/bash:5.2-alpine3.18 as runner

WORKDIR /app

COPY --from=builder /app/doh .

COPY ./web ./web

EXPOSE 443

ENTRYPOINT ["doh"]
