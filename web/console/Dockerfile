FROM node:22-alpine AS build

WORKDIR /app
COPY web/console/package.json web/console/package-lock.json ./
RUN npm ci
COPY web/console ./
RUN npm run build

FROM nginx:1.27-alpine

COPY web/console/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 8080
