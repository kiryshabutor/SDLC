# Образ веб-клиента (DV-014): сборка Vue → Caddy 2.8 с Caddyfile.
FROM node:22-alpine AS build
WORKDIR /web
COPY package.json package-lock.json .npmrc ./
RUN npm ci
COPY . .
RUN npm run build

FROM caddy:2.8-alpine
COPY --from=build /web/dist /srv
