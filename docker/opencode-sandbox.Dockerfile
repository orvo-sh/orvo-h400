FROM mirror.gcr.io/library/node:25-alpine

RUN apk add --no-cache ca-certificates git openssh-client

RUN npm install -g opencode-ai@1.2.10 --no-audit --no-fund
RUN opencode --version

WORKDIR /workspace
