package com.logaudit.license.core;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;

import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.util.Base64;

/**
 * 轻量 JWT（HS256）校验：复用平台 Go 后端同一把 JWT_SECRET，
 * 仅依赖 JDK 标准库，避免引入额外安全框架，保证授权接口的等保三级访问控制。
 */
public final class JwtVerifier {

  private static final ObjectMapper OM = new ObjectMapper();

  private JwtVerifier() {
  }

  public static class AuthException extends Exception {
    public AuthException(String message) {
      super(message);
    }
  }

  /** 校验并解析 access token，返回载荷节点 */
  public static JsonNode verify(String token, String secret) throws AuthException {
    if (token == null || token.isBlank()) {
      throw new AuthException("缺少访问令牌");
    }
    String[] parts = token.split("\\.");
    if (parts.length != 3) {
      throw new AuthException("令牌格式无效");
    }
    String signingInput = parts[0] + "." + parts[1];
    String expected;
    try {
      Mac mac = Mac.getInstance("HmacSHA256");
      mac.init(new SecretKeySpec(secret.getBytes(StandardCharsets.UTF_8), "HmacSHA256"));
      expected = Base64.getUrlEncoder().withoutPadding()
          .encodeToString(mac.doFinal(signingInput.getBytes(StandardCharsets.UTF_8)));
    } catch (Exception e) {
      throw new AuthException("令牌签名校验失败");
    }
    if (!MessageDigest.isEqual(expected.getBytes(StandardCharsets.UTF_8),
        parts[2].getBytes(StandardCharsets.UTF_8))) {
      throw new AuthException("令牌签名校验失败（密钥不一致）");
    }
    JsonNode payload;
    try {
      payload = OM.readTree(Base64.getUrlDecoder().decode(parts[1]));
    } catch (Exception e) {
      throw new AuthException("令牌载荷解析失败");
    }
    long exp = payload.path("exp").asLong(0);
    if (exp > 0 && System.currentTimeMillis() / 1000 > exp) {
      throw new AuthException("令牌已过期");
    }
    return payload;
  }
}
