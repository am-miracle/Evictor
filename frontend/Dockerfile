FROM node:22-alpine AS dependencies
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci

FROM dependencies AS builder
COPY . .
RUN npm run build

FROM dependencies AS dev
COPY . .
EXPOSE 3000
CMD ["npm", "run", "dev"]

FROM node:22-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
RUN addgroup -S evictor && adduser -S -G evictor evictor
COPY --from=builder --chown=evictor:evictor /app/.next/standalone ./
COPY --from=builder --chown=evictor:evictor /app/.next/static ./.next/static
USER evictor
EXPOSE 3000
CMD ["node", "server.js"]
