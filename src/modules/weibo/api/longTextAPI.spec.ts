/// <reference types="@jest/globals" />
import { describe, expect, test } from '@jest/globals';

const jestAny = jest as any;

jestAny.mock("./common", () => {
  const actual = jestAny.requireActual("./common") as any;
  return {
    ...actual,
    requestWithRetry: jestAny.fn(),
  };
});

import { createLongTextAPI } from "./longTextAPI";
import { requestWithRetry } from "./common";

const testDataList = [{
  id: '5093426468489917',
  text: '中微子是宇宙形成之初就存在的最古老也最原始的基本粒子',
}, {
  id: '5093698779485067',
  text: '无论如何，「AI God」的拍卖再次引发了人们对传统艺术与数字艺术的思考',
}];

describe ('Weibo API: longTextAPI', () => {
  test('fetch long text', async () => {
    const { getWeiboLongText } = createLongTextAPI();
    for (const data of testDataList) {
      (requestWithRetry as any).mockResolvedValueOnce({ data: { data: { longTextContent: data.text } } });
      const resData = await getWeiboLongText(data.id);
      expect(resData).toContain(data.text);
    }
  });
});
