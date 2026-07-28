FROM mirror.gcr.io/library/node:25-alpine

RUN apk add --no-cache ca-certificates git openssh-client

RUN npm install -g opencode-ai@1.18.8 --no-audit --no-fund
RUN opencode --version

WORKDIR /workspace
