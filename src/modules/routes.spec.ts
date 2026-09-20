/// <reference types="@jest/globals" />
import { describe, expect, test } from '@jest/globals';
import Koa from 'koa';
import Router from '@koa/router';
import { registerRoutes } from './routes';
import { RSSKoaState, RSSKoaContext } from '../types';

describe('Routes', () => {
  test('registerRoutes adds /admin/cache-stats', async () => {
    const router = new Router<RSSKoaState, RSSKoaContext>();
    registerRoutes(router);

    const allRoutes: any[] = (router as any).stack
      .filter((r: any) => r.methods)
      .map((r: any) => ({ path: r.path, methods: r.methods }));
    const matched = allRoutes.find((r: any) => {
      const methods = Array.isArray(r.methods) ? r.methods : [r.methods];
      return methods.some((m: any) => String(m).toLowerCase() === 'get') && r.path === '/admin/cache-stats';
    });
    expect(matched).toBeDefined();
  });

  test('/admin/cache-stats returns cache stats', async () => {
    const app = new Koa<RSSKoaState, RSSKoaContext>();
    const router = new Router<RSSKoaState, RSSKoaContext>();
    registerRoutes(router);

    const fakeStats = { size: 1, max: 100, calculatedSize: 1 };
    app.use(async (ctx, next) => {
      (ctx as any).cache = {
        getStats: () => fakeStats,
      };
      await next();
    });
    app.use(router.routes());

    const response: any = { statusCode: 200, body: null };
    app.use(async (ctx) => {
      response.statusCode = ctx.status;
      response.body = ctx.body;
    });

    await new Promise<void>((resolve) => {
      const handler = app.callback();
      handler({
        method: 'GET',
        url: '/admin/cache-stats',
        headers: { host: 'm.weibo.cn' },
        httpVersion: '1.1',
        connection: { remoteAddress: '127.0.0.1' },
        socket: { remoteAddress: '127.0.0.1' },
        setTimeout: () => {},
        setHeader: () => {},
        getHeader: () => ({}),
        removeHeader: () => {},
        write: () => {},
        writeHead: () => {},
        end: (chunk?: any) => {
          if (chunk && response.body === null) {
            response.body = chunk;
          }
          setImmediate(resolve);
        },
        on: () => ({}),
        once: () => ({}),
        emit: () => {},
      } as any, {
        statusCode: 200,
        setHeader: () => {},
        getHeader: () => ({}),
        writeHead: () => {},
        end: (chunk?: any) => {
          if (chunk && response.body === null) {
            response.body = chunk;
          }
          setImmediate(resolve);
        },
        removeHeader: () => {},
        on: () => ({}),
        once: () => ({}),
        emit: () => {},
      } as any);
    });

    expect(response.statusCode).toBe(200);
    const body = JSON.parse(response.body);
    expect(body.success).toBe(true);
    expect(body.stats).toEqual(fakeStats);
  });
});
