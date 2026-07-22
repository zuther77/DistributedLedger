# multtistage build

#--------------  Stage 1: compile -------------------
FROM golang:1.26-alpine AS build

WORKDIR /src

# Dependency layer 
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# SERVICE is a build-arg from Compose:
#   order-api | matching-engine | settlement-worker
ARG SERVICE
RUN test -n "$SERVICE" || (echo "SERVICE build-arg required" && exit 1 )

RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/app ./cmd/${SERVICE}


# ----- Stage 2: runtime ---------
FROM alpine:3.20

# CA cert: needed if the binary ever dials TLS endpoint
RUN apk add --no-cache ca-certificates

# copying only binary
COPY --from=build /bin/app /usr/local/bin/app

ENTRYPOINT [ "/usr/local/bin/app" ]