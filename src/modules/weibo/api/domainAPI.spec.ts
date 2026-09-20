import { describe, expect, test } from "@jest/globals";

const jestAny = jest as any;

jestAny.mock("axios", () => ({
  create: jestAny.fn(() => ({
    request: jestAny.fn(),
  })),
  default: {
    create: jestAny.fn(),
  },
}));

const mockAxios = require("axios");

// 必须在 createDomainAPI() 之前设置 mock 返回值
const mockInstance = jestAny.fn();
mockInstance.request = jestAny.fn();
mockAxios.create.mockReturnValue(mockInstance);

const { createDomainAPI } = require("./domainAPI");

const testDomainMapList = [
  {
    domain: 'kaifulee',
    uid: '1197161814',
  },
  {
    domain: 'taobao',
    uid: '1682454721',
  },
  {
    domain: 'tmall',
    uid: '1768198384',
  },
];

describe('Domain API: domain to uid', () => {
  test('basic domain format', async () => {
    const { getUIDByDomain } = createDomainAPI();
    for (const data of testDomainMapList) {
      mockInstance.mockResolvedValueOnce({
        data: {},
        request: { path: `/u/${data.uid}` },
      });
      const resData = await getUIDByDomain(data.domain);
      expect(resData).toBe(data.uid);
    }
  });
});
