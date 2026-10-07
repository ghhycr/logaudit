package com.logaudit.license.web;

import com.logaudit.license.core.HardwareFingerprint;
import com.logaudit.license.core.JwtVerifier;
import com.logaudit.license.core.LicenseCodec;
import com.logaudit.license.core.LicenseCodec.LicenseException;
import com.logaudit.license.core.LicenseCodec.LicenseInfo;
import com.logaudit.license.data.LicenseRepository;
import com.fasterxml.jackson.databind.JsonNode;
import jakarta.servlet.http.HttpServletRequest;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.http.HttpHeaders;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.LocalDate;
import java.time.LocalDateTime;
import java.time.format.DateTimeFormatter;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * 授权管理接口（Java 部署，Nginx 反代 /api/v1/license/ → 本服务）
 *
 * 授权码统一由本地离线工具 LicenseTool.exe（签名密钥 LICENSE_SECRET）签发，
 * 本服务仅负责：硬件指纹展示、授权码/授权文件校验、导入激活与导出留痕。
 *
 * GET    /health        健康检查（无需鉴权）
 * GET    /hardware      服务器硬件指纹与硬件明细
 * GET    /status        当前授权状态
 * POST   /verify        校验授权码 / 授权文件内容（不落库）
 * POST   /activate      导入并激活授权码（校验硬件绑定）
 * POST   /deactivate    解除当前授权
 * GET    /records       授权导入记录（等保三级留痕）
 * GET    /export/{id}   导出指定导入记录的 .lic 授权文件
 * GET    /export-active 导出当前生效授权的 .lic 授权文件
 */
@RestController
public class LicenseController {

  private final LicenseRepository repo;

  @Value("${license.secret}")
  private String secret;

  @Value("${license.product}")
  private String product;

  @Value("${license.data-dir}")
  private String dataDir;

  @Value("${security.jwt-secret}")
  private String jwtSecret;

  private static final DateTimeFormatter FMT = DateTimeFormatter.ofPattern("yyyy-MM-dd HH:mm:ss");

  public LicenseController(LicenseRepository repo) {
    this.repo = repo;
  }

  // ---------------- 健康检查 ----------------

  @GetMapping("/health")
  public ApiResp<Map<String, Object>> health() {
    Map<String, Object> m = new LinkedHashMap<>();
    m.put("status", "up");
    m.put("service", "license-service");
    m.put("time", LocalDateTime.now().format(FMT));
    return ApiResp.ok(m);
  }

  // ---------------- 硬件指纹 ----------------

  @GetMapping("/hardware")
  public ResponseEntity<ApiResp<Map<String, Object>>> hardware(HttpServletRequest req) {
    try {
      requireAuth(req, false);
    } catch (JwtVerifier.AuthException e) {
      return unauth(e.getMessage());
    }
    Map<String, Object> m = new LinkedHashMap<>();
    m.put("hardware_id", HardwareFingerprint.fingerprint());
    m.put("details", HardwareFingerprint.collect());
    return ResponseEntity.ok(ApiResp.ok(m));
  }

  // ---------------- 当前授权状态 ----------------

  @GetMapping("/status")
  public ResponseEntity<ApiResp<Map<String, Object>>> status(HttpServletRequest req) {
    try {
      requireAuth(req, false);
    } catch (JwtVerifier.AuthException e) {
      return unauth(e.getMessage());
    }
    String hw = HardwareFingerprint.fingerprint();
    Map<String, Object> m = new LinkedHashMap<>();
    m.put("hardware_id", hw);
    m.put("product", product);

    Map<String, Object> row = safeLoadActive();
    if (row == null) {
      m.put("activated", false);
      m.put("status", "none");
      m.put("status_text", "未授权");
      m.put("license_code", "");
      m.put("bound_matched", false);
      return ResponseEntity.ok(ApiResp.ok(m));
    }
    String code = str(row.get("license_code"));
    boolean permanent = intv(row.get("permanent")) == 1;
    String expire = str(row.get("expire_date"));
    boolean boundMatched = hw.equals(str(row.get("hardware_id")));

    m.put("activated", true);
    m.put("license_code", code);
    m.put("license_code_masked", LicenseCodec.mask(code));
    m.put("permanent", permanent);
    m.put("hardware_bound", str(row.get("hardware_id")));
    m.put("bound_matched", boundMatched);
    m.put("activated_by", str(row.get("activated_by")));
    m.put("activated_at", row.get("activated_at") == null ? "" : row.get("activated_at").toString());

    // 实时复核有效期（持久化记录 + 当前时间双重校验）
    try {
      LicenseInfo info = LicenseCodec.parse(code, secret);
      m.put("status", info.permanent ? "permanent" : "active");
      m.put("status_text", info.permanent ? "永久授权" : "限时授权");
      m.put("expire_date", info.expire);
      m.put("days_left", info.daysLeft);
      m.put("issued_at", LocalDateTime.ofEpochSecond(info.issuedAt, 0, java.time.ZoneOffset.ofHours(8)).format(FMT));
      if (!boundMatched) {
        m.put("status", "mismatch");
        m.put("status_text", "硬件不匹配");
      }
    } catch (LicenseException e) {
      m.put("status", "invalid");
      m.put("status_text", "授权无效（" + e.getMessage() + "）");
      m.put("expire_date", permanent ? "永久有效" : expire);
      m.put("days_left", -1);
    }
    return ResponseEntity.ok(ApiResp.ok(m));
  }

  // ---------------- 校验（不落库） ----------------

  @PostMapping("/verify")
  public ResponseEntity<ApiResp<Map<String, Object>>> verify(@RequestBody Map<String, Object> body,
                                                             HttpServletRequest req) {
    try {
      requireAuth(req, false);
    } catch (JwtVerifier.AuthException e) {
      return unauth(e.getMessage());
    }
    String input;
    try {
      input = extractCode(body);
      LicenseInfo info = LicenseCodec.parse(input, secret);
      String hw = HardwareFingerprint.fingerprint();
      boolean matched = hw.equals(info.hardwareId);
      Map<String, Object> m = new LinkedHashMap<>();
      m.put("valid", true);
      m.put("matched", matched);
      m.put("message", matched ? "校验通过，授权码与本机硬件匹配" : "授权码有效，但绑定硬件与本机不一致");
      m.put("hardware_id", info.hardwareId);
      m.put("local_hardware_id", hw);
      m.put("permanent", info.permanent);
      m.put("expire", info.expire);
      m.put("days_left", info.daysLeft);
      m.put("issued_at", LocalDateTime.ofEpochSecond(info.issuedAt, 0, java.time.ZoneOffset.ofHours(8)).format(FMT));
      m.put("code", info.code);
      return ResponseEntity.ok(ApiResp.ok(m));
    } catch (LicenseException e) {
      return ResponseEntity.badRequest().body(ApiResp.fail(e.getMessage()));
    }
  }

  // ---------------- 激活（导入授权码 / 授权文件） ----------------

  @PostMapping("/activate")
  public ResponseEntity<ApiResp<Map<String, Object>>> activate(@RequestBody Map<String, Object> body,
                                                               HttpServletRequest req) {
    String operator;
    try {
      operator = requireAuth(req, true);
    } catch (JwtVerifier.AuthException e) {
      return unauth(e.getMessage());
    }
    String input;
    try {
      input = extractCode(body);
    } catch (LicenseException e) {
      return ResponseEntity.badRequest().body(ApiResp.fail(e.getMessage()));
    }
    LicenseInfo info;
    try {
      info = LicenseCodec.parse(input, secret);
    } catch (LicenseException e) {
      return ResponseEntity.badRequest().body(ApiResp.fail(e.getMessage()));
    }
    String hw = HardwareFingerprint.fingerprint();
    if (!hw.equals(info.hardwareId)) {
      return ResponseEntity.badRequest().body(ApiResp.fail(
          "授权码绑定的硬件指纹与本机不一致，无法在本机激活（本机：" + hw + "，授权码：" + info.hardwareId + "）"));
    }
    repo.saveActive(info.hardwareId, info.code, info.permanent, info.expire, info.issuedAt, operator);
    writeBackup(info);
    // 导入留痕（等保三级：授权操作可追溯，便于事后重新导出 .lic）
    String customer = str(body.get("customer"));
    String note = str(body.get("note"));
    repo.insertRecord(info.hardwareId, info.code, info.permanent, info.expire,
        licenseDays(info), customer, note.isBlank() ? "LicenseTool 导入激活" : note, operator);

    Map<String, Object> m = new LinkedHashMap<>();
    m.put("activated", true);
    m.put("status", info.permanent ? "permanent" : "active");
    m.put("status_text", info.permanent ? "永久授权" : "限时授权");
    m.put("hardware_id", info.hardwareId);
    m.put("expire", info.expire);
    m.put("days_left", info.daysLeft);
    m.put("code", info.code);
    return ResponseEntity.ok(ApiResp.ok(m));
  }

  @PostMapping("/deactivate")
  public ResponseEntity<ApiResp<Map<String, Object>>> deactivate(HttpServletRequest req) {
    try {
      requireAuth(req, true);
    } catch (JwtVerifier.AuthException e) {
      return unauth(e.getMessage());
    }
    repo.clearActive();
    try {
      Files.deleteIfExists(Path.of(dataDir, "active.lic"));
    } catch (Exception ignored) {
      // 备份文件不存在时忽略
    }
    Map<String, Object> m = new LinkedHashMap<>();
    m.put("activated", false);
    m.put("status", "none");
    return ResponseEntity.ok(ApiResp.ok(m));
  }

  // ---------------- 授权导入记录 ----------------

  @GetMapping("/records")
  public ResponseEntity<ApiResp<Map<String, Object>>> records(HttpServletRequest req,
                                                              @RequestParam(defaultValue = "50") int limit) {
    try {
      requireAuth(req, true);
    } catch (JwtVerifier.AuthException e) {
      return unauth(e.getMessage());
    }
    List<Map<String, Object>> rows = repo.listRecords(Math.min(Math.max(limit, 1), 500));
    for (Map<String, Object> r : rows) {
      r.put("license_code_masked", LicenseCodec.mask(str(r.get("license_code"))));
      r.put("license_code", "");
    }
    Map<String, Object> m = new LinkedHashMap<>();
    m.put("total", rows.size());
    m.put("list", rows);
    return ResponseEntity.ok(ApiResp.ok(m));
  }

  // ---------------- 导出授权文件 ----------------

  @GetMapping("/export/{id}")
  public ResponseEntity<?> exportById(@PathVariable long id, HttpServletRequest req) {
    try {
      requireAuth(req, true);
    } catch (JwtVerifier.AuthException e) {
      return unauth(e.getMessage());
    }
    Map<String, Object> row = repo.findRecord(id);
    if (row == null) {
      return ResponseEntity.badRequest().body(ApiResp.fail("导入记录不存在"));
    }
    String code = str(row.get("license_code"));
    LicenseInfo info;
    try {
      info = LicenseCodec.parse(code, secret);
    } catch (LicenseException e) {
      return ResponseEntity.badRequest().body(ApiResp.fail(e.getMessage()));
    }
    String content = LicenseCodec.buildLicFileContent(code, info, product);
    String filename = "logaudit-" + info.hardwareId + ".lic";
    return licResponse(content, filename);
  }

  @GetMapping("/export-active")
  public ResponseEntity<?> exportActive(HttpServletRequest req) {
    try {
      requireAuth(req, true);
    } catch (JwtVerifier.AuthException e) {
      return unauth(e.getMessage());
    }
    Map<String, Object> row = safeLoadActive();
    if (row == null) {
      return ResponseEntity.badRequest().body(ApiResp.fail("当前没有生效的授权，无法导出"));
    }
    String code = str(row.get("license_code"));
    LicenseInfo info;
    try {
      info = LicenseCodec.parse(code, secret);
    } catch (LicenseException e) {
      return ResponseEntity.badRequest().body(ApiResp.fail(e.getMessage()));
    }
    String content = LicenseCodec.buildLicFileContent(code, info, product);
    return licResponse(content, "logaudit-active-" + info.hardwareId + ".lic");
  }

  // ---------------- 内部方法 ----------------

  /** 从请求体中提取授权码：支持 {code} 直接授权码，或 {file_content}/.lic 文件内容 */
  private String extractCode(Map<String, Object> body) throws LicenseException {
    if (body == null) {
      throw new LicenseException("请求体为空");
    }
    String code = str(body.get("code"));
    if (!code.isBlank()) {
      return code;
    }
    String fileContent = str(body.get("file_content"));
    if (!fileContent.isBlank()) {
      return LicenseCodec.readCodeFromContent(fileContent);
    }
    throw new LicenseException("请提供授权码或授权文件内容");
  }

  private ResponseEntity<ApiResp<Map<String, Object>>> unauth(String msg) {
    return ResponseEntity.status(401).body(ApiResp.fail(msg));
  }

  /** 鉴权：解析 Bearer JWT，adminOnly=true 时要求角色为 admin；返回操作者用户名 */
  private String requireAuth(HttpServletRequest req, boolean adminOnly) throws JwtVerifier.AuthException {
    String header = req.getHeader(HttpHeaders.AUTHORIZATION);
    String token = null;
    if (header != null && header.toLowerCase().startsWith("bearer ")) {
      token = header.substring(7).trim();
    }
    JsonNode payload = JwtVerifier.verify(token, jwtSecret);
    String role = payload.path("role").asText("");
    if (adminOnly && !"admin".equalsIgnoreCase(role)) {
      throw new JwtVerifier.AuthException("仅管理员可执行该操作");
    }
    return payload.path("username").asText(payload.path("sub").asText(""));
  }

  private ResponseEntity<?> licResponse(String content, String filename) {
    return ResponseEntity.ok()
        .header(HttpHeaders.CONTENT_TYPE, "text/plain; charset=utf-8")
        .header(HttpHeaders.CONTENT_DISPOSITION,
            "attachment; filename=\"" + filename + "\"; filename*=UTF-8''" + filename)
        .body(content.getBytes(StandardCharsets.UTF_8));
  }

  private Map<String, Object> safeLoadActive() {
    try {
      return repo.loadActive();
    } catch (Exception e) {
      return null;
    }
  }

  /** 激活后同步写一份授权文件到数据卷（数据库异常时仍可离线核验） */
  private void writeBackup(LicenseInfo info) {
    try {
      Files.createDirectories(Path.of(dataDir));
      Files.writeString(Path.of(dataDir, "active.lic"),
          LicenseCodec.buildLicFileContent(info.code, info, product), StandardCharsets.UTF_8);
    } catch (Exception ignored) {
      // 数据卷不可写时忽略，不影响主流程
    }
  }

  /** 由授权码反推授权总天数（永久授权返回 0），仅用于留痕展示 */
  private static int licenseDays(LicenseInfo info) {
    if (info.permanent) {
      return 0;
    }
    try {
      long start = LocalDate.ofInstant(java.time.Instant.ofEpochSecond(info.issuedAt),
          java.time.ZoneOffset.ofHours(8)).toEpochDay();
      long end = LocalDate.parse(info.expire).toEpochDay();
      return (int) Math.max(end - start, 0);
    } catch (Exception e) {
      return 0;
    }
  }

  private static String str(Object o) {
    return o == null ? "" : String.valueOf(o);
  }

  private static int intv(Object o) {
    if (o == null) {
      return 0;
    }
    if (o instanceof Number n) {
      return n.intValue();
    }
    try {
      return Integer.parseInt(String.valueOf(o));
    } catch (Exception e) {
      return 0;
    }
  }
}
