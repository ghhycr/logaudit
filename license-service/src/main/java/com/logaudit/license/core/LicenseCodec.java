package com.logaudit.license.core;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;

import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.time.Duration;
import java.time.LocalDate;
import java.time.LocalDateTime;
import java.util.Base64;

/**
 * 授权码编解码（与 LicenseTool.exe / 平台前端完全一致的协议）
 *
 * 格式： LGA2.&lt;base64url(payload)&gt;.&lt;hex(hmac_sha256(secret, base64url(payload)))&gt;
 * 载荷： {"hw": "&lt;硬件指纹&gt;", "perm": true|false, "iat": &lt;unix&gt;, "exp": "YYYY-MM-DD"}
 */
public final class LicenseCodec {

  public static final String PREFIX = "LGA2.";

  private static final ObjectMapper OM = new ObjectMapper();

  private LicenseCodec() {
  }

  /** 授权码校验异常（携带中文原因，直接回显前端） */
  public static class LicenseException extends Exception {
    public LicenseException(String message) {
      super(message);
    }
  }

  /** 解析后的授权信息 */
  public static class LicenseInfo {
    public String hardwareId;
    public boolean permanent;
    /** 到期时间："YYYY-MM-DD" 或 "永久有效" */
    public String expire;
    /** 签发时间（unix 秒） */
    public long issuedAt;
    /** 剩余天数；-1 表示永久 */
    public long daysLeft;
    public String code;
  }

  /** 签发授权码：绑定硬件指纹，限时（days 天）或永久 */
  public static String build(String hardwareId, Integer days, boolean permanent, String secret) {
    long iat = System.currentTimeMillis() / 1000;
    StringBuilder json = new StringBuilder();
    json.append("{\"hw\": ").append(q(hardwareId))
        .append(", \"perm\": ").append(permanent)
        .append(", \"iat\": ").append(iat);
    if (!permanent) {
      int d = (days == null || days <= 0) ? 30 : days;
      json.append(", \"exp\": ").append(q(LocalDate.now().plusDays(d).toString()));
    }
    json.append("}");
    String b64 = b64urlEncode(json.toString().getBytes(StandardCharsets.UTF_8));
    return PREFIX + b64 + "." + hmacHex(b64, secret);
  }

  /** 校验授权码：签名 → 载荷 → 硬件绑定信息 → 有效期 */
  public static LicenseInfo parse(String code, String secret) throws LicenseException {
    String c = code == null ? "" : code.trim();
    if (!c.startsWith(PREFIX)) {
      throw new LicenseException("授权码前缀无效（应以 LGA2. 开头）");
    }
    String body = c.substring(PREFIX.length());
    int dot = body.lastIndexOf('.');
    if (dot <= 0) {
      throw new LicenseException("授权码格式无效");
    }
    String b64 = body.substring(0, dot);
    String sig = body.substring(dot + 1);

    if (!constantTimeEquals(hmacHex(b64, secret), sig)) {
      throw new LicenseException("授权码签名校验失败（密钥不一致或被篡改）");
    }
    byte[] raw;
    try {
      raw = Base64.getUrlDecoder().decode(b64);
    } catch (Exception e) {
      throw new LicenseException("授权码编码无效");
    }
    JsonNode p;
    try {
      p = OM.readTree(raw);
    } catch (Exception e) {
      throw new LicenseException("授权码载荷解析失败");
    }
    String hw = p.path("hw").asText("");
    if (hw.isEmpty()) {
      throw new LicenseException("授权码缺少硬件绑定信息");
    }

    LicenseInfo li = new LicenseInfo();
    li.code = c;
    li.hardwareId = hw;
    li.permanent = p.path("perm").asBoolean(false);
    li.issuedAt = p.path("iat").asLong(0);

    LocalDateTime now = LocalDateTime.now();
    if (li.permanent) {
      li.expire = "永久有效";
      li.daysLeft = -1;
    } else {
      String exp = p.path("exp").asText("");
      if (exp.isEmpty()) {
        throw new LicenseException("授权码缺少有效期");
      }
      LocalDate d;
      try {
        d = LocalDate.parse(exp);
      } catch (Exception e) {
        throw new LicenseException("授权码有效期格式无效");
      }
      LocalDateTime expDt = d.atTime(23, 59, 59);
      if (now.isAfter(expDt)) {
        throw new LicenseException("授权码已过期（" + exp + "）");
      }
      li.expire = exp;
      li.daysLeft = Duration.between(now, expDt).toDays();
    }
    return li;
  }

  /** 从授权文件（.lic）内容中提取第一行授权码 */
  public static String readCodeFromContent(String content) throws LicenseException {
    if (content == null || content.isBlank()) {
      throw new LicenseException("授权文件内容为空");
    }
    for (String line : content.split("\\R")) {
      String t = line.trim();
      if (t.startsWith(PREFIX)) {
        return t;
      }
    }
    throw new LicenseException("授权文件中未找到有效授权码（应以 LGA2. 开头）");
  }

  /** 生成 .lic 授权文件内容（首行为授权码，其后为元信息注释） */
  public static String buildLicFileContent(String code, LicenseInfo info, String product) {
    StringBuilder sb = new StringBuilder();
    sb.append(code.trim()).append('\n');
    sb.append("#PRODUCT: ").append(product).append('\n');
    sb.append("#HW: ").append(info.hardwareId).append('\n');
    sb.append("#TYPE: ").append(info.permanent ? "永久授权" : "限时授权").append('\n');
    sb.append("#EXPIRE: ").append(info.expire).append('\n');
    if (info.issuedAt > 0) {
      sb.append("#ISSUED_AT: ")
          .append(LocalDateTime.ofEpochSecond(info.issuedAt, 0, java.time.ZoneOffset.ofHours(8)))
          .append('\n');
    }
    sb.append("#EXPORTED_AT: ").append(LocalDateTime.now()).append('\n');
    return sb.toString();
  }

  /** 授权码脱敏展示（保留前缀与尾部 6 位） */
  public static String mask(String code) {
    if (code == null || code.length() <= 16) {
      return code == null ? "" : code;
    }
    return code.substring(0, 10) + "********" + code.substring(code.length() - 6);
  }

  // ---------------- 基础工具 ----------------

  public static String b64urlEncode(byte[] data) {
    return Base64.getUrlEncoder().withoutPadding().encodeToString(data);
  }

  public static String hmacHex(String data, String secret) {
    try {
      Mac mac = Mac.getInstance("HmacSHA256");
      mac.init(new SecretKeySpec(secret.getBytes(StandardCharsets.UTF_8), "HmacSHA256"));
      return hex(mac.doFinal(data.getBytes(StandardCharsets.UTF_8)));
    } catch (Exception e) {
      throw new IllegalStateException("HMAC-SHA256 不可用", e);
    }
  }

  public static String sha256Hex(String data) {
    try {
      return hex(MessageDigest.getInstance("SHA-256").digest(data.getBytes(StandardCharsets.UTF_8)));
    } catch (Exception e) {
      throw new IllegalStateException("SHA-256 不可用", e);
    }
  }

  private static String hex(byte[] b) {
    StringBuilder sb = new StringBuilder(b.length * 2);
    for (byte x : b) {
      sb.append(Character.forDigit((x >> 4) & 0xF, 16)).append(Character.forDigit(x & 0xF, 16));
    }
    return sb.toString();
  }

  private static boolean constantTimeEquals(String a, String b) {
    if (a == null || b == null) {
      return false;
    }
    return MessageDigest.isEqual(a.getBytes(StandardCharsets.UTF_8), b.getBytes(StandardCharsets.UTF_8));
  }

  private static String q(String s) {
    StringBuilder sb = new StringBuilder("\"");
    for (int i = 0; i < s.length(); i++) {
      char c = s.charAt(i);
      switch (c) {
        case '"' -> sb.append("\\\"");
        case '\\' -> sb.append("\\\\");
        case '\n' -> sb.append("\\n");
        case '\r' -> sb.append("\\r");
        case '\t' -> sb.append("\\t");
        default -> {
          if (c < 0x20) {
            sb.append(String.format("\\u%04x", (int) c));
          } else {
            sb.append(c);
          }
        }
      }
    }
    return sb.append('"').toString();
  }
}
