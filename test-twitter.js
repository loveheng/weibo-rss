const axios = require('axios');
const https = require('https');

// 配置
const BEARER_TOKEN = 'd3f321513fc7f7bd65074f0dd699cefffaffaaf9';
const PROXY_HOST = '192.168.1.40';
const PROXY_PORT = 2080;
const TEST_USERNAME = 'Fides_Ascensio';

// 创建 axios 实例，使用代理
const instance = axios.create({
  timeout: 10000,
  headers: {
    'User-Agent': 'Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Mobile Safari/537.36',
    'Authorization': `Bearer ${BEARER_TOKEN}`,
  },
  httpAgent: new https.Agent(),
  httpsAgent: new https.Agent(),
  proxy: {
    host: PROXY_HOST,
    port: PROXY_PORT,
    protocol: 'http'
  }
});

// 测试 1: 获取用户信息
async function testGetUserInfo() {
  console.log('测试 1: 获取 Twitter 用户信息...');
  try {
    const response = await instance.get(`https://api.twitter.com/2/users/by/username/${TEST_USERNAME}`, {
      params: {
        'user.fields': 'description,profile_image_url,public_metrics'
      }
    });
    console.log('✅ 成功获取用户信息:');
    console.log(JSON.stringify(response.data, null, 2));
    return response.data.data;
  } catch (error) {
    console.error('❌ 获取用户信息失败:', error.message);
    if (error.response) {
      console.error('响应状态:', error.response.status);
      console.error('响应数据:', error.response.data);
    }
    return null;
  }
}

// 测试 2: 获取用户推文
async function testGetUserTweets(userId) {
  console.log('\n测试 2: 获取用户推文...');
  try {
    const response = await instance.get(`https://api.twitter.com/2/users/${userId}/tweets`, {
      params: {
        'tweet.fields': 'created_at,public_metrics,entities',
        'exclude': 'retweets,replies',
        max_results: 5
      }
    });
    console.log('✅ 成功获取推文:');
    console.log(JSON.stringify(response.data, null, 2));
    return response.data.data;
  } catch (error) {
    console.error('❌ 获取推文失败:', error.message);
    if (error.response) {
      console.error('响应状态:', error.response.status);
      console.error('响应数据:', error.response.data);
    }
    return null;
  }
}

// 主测试函数
async function runTests() {
  console.log('🚀 开始测试 Twitter API 代理配置...');
  console.log(`代理: ${PROXY_HOST}:${PROXY_PORT}`);
  console.log(`Bearer Token: ${BEARER_TOKEN.substring(0, 10)}...`);
  console.log(`测试用户: ${TEST_USERNAME}\n`);

  const userInfo = await testGetUserInfo();
  if (userInfo) {
    await testGetUserTweets(userInfo.id);
  }

  console.log('\n✅ 测试完成');
}

// 运行测试
runTests().catch(console.error);
