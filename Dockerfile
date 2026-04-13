# Dockerfile — obsidian-tasks-board standalone
#
# Build:
#   docker build -t obsidian-tasks-board .
#
# Board interativo (vault montado como volume):
#   docker run -it --rm -v /path/to/vault:/vault obsidian-tasks-board board
#
# Init de novo vault:
#   docker run -it --rm -v /path/to/output:/output obsidian-tasks-board init --name "Meu Vault" --dir /output
#
# O vault deve ser montado em /vault. O board detecta automaticamente.

FROM node:22-slim

WORKDIR /app

COPY package.json ./
COPY cli.ts ./
COPY src/ ./src/

VOLUME ["/vault"]

ENTRYPOINT ["node", "--experimental-strip-types", "/app/cli.ts"]
CMD ["board", "--vault", "/vault"]
