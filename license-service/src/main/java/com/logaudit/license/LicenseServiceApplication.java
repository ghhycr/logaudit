package com.logaudit.license;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

/**
 * 日志审计平台 - 授权管理模块（Java 部署）
 *
 * 能力：
 *  - 采集服务器硬件指纹（machine-id / DMI product_uuid / 主板序列号 / 网卡 MAC）
 *  - 授权码签发与校验：LGA2.&lt;base64url(payload)&gt;.&lt;hex(HMAC-SHA256)&gt;，与 LicenseTool.exe 完全兼容
 *  - 授权文件（.lic）导入校验、复制授权码、导出授权文件
 *  - 签发历史留痕（MySQL audit_meta）
 */
@SpringBootApplication
public class LicenseServiceApplication {
  public static void main(String[] args) {
    SpringApplication.run(LicenseServiceApplication.class, args);
  }
}
