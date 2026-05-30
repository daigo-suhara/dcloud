FROM node:22-alpine AS build

WORKDIR /app
COPY services/hello-world/package.json ./package.json
COPY services/hello-world/build.mjs ./build.mjs
COPY services/hello-world/index.html ./index.html
COPY services/hello-world/assets ./assets
RUN node build.mjs

FROM nginx:1.27-alpine

COPY services/hello-world/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html

EXPOSE 8080
