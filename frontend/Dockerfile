# ---- deps stage ----
FROM oven/bun:1-alpine AS deps
WORKDIR /app

COPY package.json bun.lock ./
RUN bun install --frozen-lockfile

# ---- dev stage (hot reload; used by docker-compose.dev.yml) ----
# Sources are bind-mounted at runtime, so nothing is copied here.
FROM deps AS dev
EXPOSE 80
CMD ["bun", "--bun", "run", "dev", "--host", "0.0.0.0", "--port", "80"]

# ---- build stage ----
FROM deps AS build
COPY . .
RUN bun --bun run build

# ---- runtime stage (default target) ----
FROM nginx:alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
