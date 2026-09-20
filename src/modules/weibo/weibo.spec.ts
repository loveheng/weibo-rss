/// <reference types="@jest/globals" />
import { describe, expect, test } from '@jest/globals';

const jestAny = jest as any;

const mockIndexAPIFunctions = {
  getIndexUserInfo: jestAny.fn(),
  getWeiboContentList: jestAny.fn(),
};

const mockLongTextAPIFunctions = {
  getWeiboLongText: jestAny.fn(),
};

const mockDetailAPIFunctions = {
  getWeiboDetail: jestAny.fn(),
};

const mockDomainAPIFunctions = {
  getUIDByDomain: jestAny.fn(),
};

jestAny.mock('./api/indexAPI', () => ({
  createIndexAPI: jestAny.fn(() => mockIndexAPIFunctions),
}));

jestAny.mock('./api/longTextAPI', () => ({
  createLongTextAPI: jestAny.fn(() => mockLongTextAPIFunctions),
}));

jestAny.mock('./api/detailAPI', () => ({
  createDetailAPI: jestAny.fn(() => mockDetailAPIFunctions),
}));

jestAny.mock('./api/domainAPI', () => ({
  createDomainAPI: jestAny.fn(() => mockDomainAPIFunctions),
}));

jestAny.mock('./api/common', () => ({
  ...jestAny.requireActual('./api/common'),
  requestWithRetry: jestAny.fn(),
}));

import { WeiboData } from "./weibo";
import { WeiboStatus } from '../../types';

const wbData = new WeiboData({
  set: async () => null,
  get: async () => null,
  memo: async <T>(cb: () => T): Promise<Awaited<T>> => await cb(),
});

describe('Weibo Data: user weibo profile and list', () => {
  const TEST_UID = '5890672121';

  test('fetch user weibo profile and list success', async () => {
    mockIndexAPIFunctions.getIndexUserInfo.mockResolvedValueOnce({
      uid: TEST_UID,
      containerId: 'test_container',
      screenName: '搜狐新闻',
      description: 'desc',
    });
    mockIndexAPIFunctions.getWeiboContentList.mockResolvedValueOnce([]);
    
    const resData = await wbData.fetchUserLatestWeibo(TEST_UID);
    expect(resData.uid).toBe(TEST_UID);
    expect(resData.screenName).toBe('搜狐新闻');
    expect(resData.statusList).toBeDefined();
  });

  test('fetch weibo with long text', async () => {
    const longText = '中微子是宇宙形成之初就存在的最古老也最原始的基本粒子';
    mockLongTextAPIFunctions.getWeiboLongText.mockResolvedValueOnce(longText);
    
    const resData = await wbData.fillStatusWithLongText({
      id: '5093426468489917',
      isLongText: true,
      text: '',
    } as WeiboStatus);
    expect(resData.text).toContain(longText);
  });
});

describe('Weibo Data: domain to uid', () => {
  test('basic domain format', async () => {
    mockDomainAPIFunctions.getUIDByDomain.mockResolvedValueOnce('1197161814');
    const resData = await wbData.fetchUIDByDomain('kaifulee');
    expect(resData).toBe('1197161814');
  });
});
