FROM node:22-alpine AS build

WORKDIR /app
COPY services/hello-world/package.json ./package.json
COPY services/hello-world/package-lock.json ./package-lock.json
RUN npm ci
COPY services/hello-world/ ./
RUN npm run build

FROM nginx:1.27-alpine

COPY services/hello-world/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/index.html /usr/share/nginx/html/index.html
COPY --from=build /app/src/styles.css /usr/share/nginx/html/styles.css
COPY --from=build /app/dist/bundle.js /usr/share/nginx/html/bundle.js

EXPOSE 8080
