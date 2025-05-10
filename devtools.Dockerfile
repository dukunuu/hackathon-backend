FROM golang:1.24.3-alpine3.21

# install all your dev-only CLIs
RUN go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest \
 && go install github.com/air-verse/air@latest \
 && go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest \
 && go install github.com/swaggo/swag/cmd/swag@latest 

# a no-op default so you can override with e.g. "sqlc", "air", "migrate"
ENTRYPOINT ["sh","-c"]
CMD ["echo 'please pass a command, e.g. sqlc generate'"]
