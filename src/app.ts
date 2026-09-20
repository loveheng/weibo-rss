import Koa from 'koa';
import Router from '@koa/router';
import serve from 'koa-static';
import path from 'path';
import config from './config';
import { logger } from "./modules/logger";
import { normalizePort } from './utils';
import { registerRoutes } from './modules/routes';
import { RSSKoaContext, RSSKoaState } from './types';
import { createMemoryCache } from './modules/cache';
import { RequestCollapsing } from './modules/requestCollapsing';
import { WeiboData } from './modules/weibo/weibo';
import { stopVisitorCookieRotation } from './modules/weibo/api/common';

const koaApp = new Koa<RSSKoaState, RSSKoaContext>();
const initApp = () => {
  const router = new Router<RSSKoaState, RSSKoaContext>();
  registerRoutes(router);

  // memoryCache
  const cache = createMemoryCache(undefined, logger);
  const requestCollapsing = new RequestCollapsing();

  // weibo
  const weiboData = new WeiboData(cache, logger);

  // enable X-Forwarded-For
  koaApp.proxy = true;

  // cache stats logging
  const logCacheStats = () => {
    try {
      const stats = (cache as any).getStats?.() || (cache as any).cache?.calculatedSize;
      logger.info(`[cache] stats=${JSON.stringify(stats)}`);
    } catch (err) {
      logger.debug(`[cache] stats unavailable`);
    }
  };
  setInterval(logCacheStats, 5 * 60 * 1000);

  // init middleware
  const app = koaApp
    .use(async (ctx, next) => {
      // duration log
      const startTime = Date.now();
      logger.debug(`${ctx.req.method} ${ctx.originalUrl} ${ctx.ip}`);
      await next();
      const duration = Date.now() - startTime;
      logger.info(`[${ctx.status}] ${ctx.req.method} ${ctx.originalUrl} ${ctx.ip} hit: ${ctx.state.hit || 0} ${duration}ms`);
    })
    .use(async (ctx, next) => {
      ctx.cache = cache;
      ctx.weibo = weiboData;
      ctx.requestCollapsing = requestCollapsing;
      await next();
    })
    .use(serve(config.rootDir + '/public'))
    .use(router.routes());

  // graceful shutdown
  const gracefulShutdown = async (signal: string) => {
    logger.info(`${signal} received, shutting down...`);
    logCacheStats();
    stopVisitorCookieRotation();
    httpServer.close(async () => {
      logger.info('HTTP server closed');
      process.exit(0);
    });
    setTimeout(() => {
      logger.error('Forced shutdown after timeout');
      process.exit(1);
    }, 10000);
  };

  return { app, gracefulShutdown };
};

// start service
const { gracefulShutdown } = initApp();
const port = normalizePort(process.env.PORT || config.port);
const httpServer = koaApp.listen(port, () => {
  logger.info(`weibo-rss start`);
  logger.info(`Listening http://0.0.0.0:${port}`);
});

process.on('SIGINT', () => gracefulShutdown('SIGINT'));
process.on('SIGTERM', () => gracefulShutdown('SIGTERM'));
