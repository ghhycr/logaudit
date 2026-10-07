package com.logaudit.license.core;

import java.io.File;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.util.ArrayList;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * 服务器硬件指纹采集（授权绑定用）
 *
 * 容器内运行：宿主机根分区以只读方式挂载到 /hostroot（与 Go 后端 hostmon 一致），
 * 依次读取 /etc/machine-id、DMI product_uuid、主板序列号、网卡 MAC（排序去重），
 * 生成稳定的 SHA-256 指纹，格式化为 LGA-XXXXXXXX-XXXXXXXX-XXXXXXXX-XXXXXXXX。
 */
public final class HardwareFingerprint {

  /** 宿主机根挂载点，可用环境变量 HOST_ROOT 覆盖；不存在时回退本机 / */
  private static final String HOST_ROOT = System.getenv().getOrDefault("HOST_ROOT", "/hostroot");

  private HardwareFingerprint() {
  }

  /** 采集硬件明细（供前端展示与排障） */
  public static Map<String, String> collect() {
    String base = new File(HOST_ROOT).isDirectory() ? HOST_ROOT : "";
    Map<String, String> m = new LinkedHashMap<>();
    m.put("hostname", firstNonBlank(read(base + "/etc/hostname"), hostName()));
    m.put("machine_id", firstNonBlank(read(base + "/etc/machine-id"), read(base + "/var/lib/dbus/machine-id")));
    m.put("product_uuid", read(base + "/sys/class/dmi/id/product_uuid"));
    m.put("product_name", read(base + "/sys/class/dmi/id/product_name"));
    m.put("board_serial", read(base + "/sys/class/dmi/id/board_serial"));
    m.put("bios_version", read(base + "/sys/class/dmi/id/bios_version"));
    m.put("mac_addresses", String.join(", ", macAddresses(base)));
    m.put("cpu_model", cpuModel(base));
    m.put("cpu_cores", String.valueOf(Runtime.getRuntime().availableProcessors()));
    return m;
  }

  /** 硬件指纹（SHA-256，分组格式化） */
  public static String fingerprint() {
    Map<String, String> d = collect();
    StringBuilder raw = new StringBuilder("logaudit|");
    raw.append(nvl(d.get("machine_id"))).append('|')
        .append(nvl(d.get("product_uuid"))).append('|')
        .append(nvl(d.get("product_name"))).append('|')
        .append(nvl(d.get("board_serial"))).append('|')
        .append(nvl(d.get("mac_addresses")));
    String hex = LicenseCodec.sha256Hex(raw.toString()).toUpperCase();
    return "LGA-" + group(hex.substring(0, 8)) + "-" + group(hex.substring(8, 16))
        + "-" + group(hex.substring(16, 24)) + "-" + group(hex.substring(24, 32));
  }

  private static String group(String s) {
    return s.toUpperCase();
  }

  private static List<String> macAddresses(String base) {
    List<String> out = new ArrayList<>();
    File net = new File(base + "/sys/class/net");
    File[] ifaces = net.listFiles();
    if (ifaces == null) {
      return out;
    }
    for (File f : ifaces) {
      String name = f.getName();
      if ("lo".equals(name) || name.startsWith("docker") || name.startsWith("br-")
          || name.startsWith("veth") || name.startsWith("virbr")) {
        continue;
      }
      String mac = read(f.getAbsolutePath() + "/address");
      if (mac != null && !mac.isBlank() && !"00:00:00:00:00:00".equals(mac.trim())) {
        out.add(mac.trim().toLowerCase());
      }
    }
    Collections.sort(out);
    return out;
  }

  private static String cpuModel(String base) {
    String cpuinfo = read(base + "/proc/cpuinfo");
    if (cpuinfo == null) {
      return "";
    }
    for (String line : cpuinfo.split("\\R")) {
      if (line.startsWith("model name")) {
        int i = line.indexOf(':');
        if (i > 0) {
          return line.substring(i + 1).trim();
        }
      }
    }
    return "";
  }

  private static String read(String path) {
    try {
      File f = new File(path);
      if (!f.exists() || f.isDirectory()) {
        return "";
      }
      String s = new String(Files.readAllBytes(f.toPath()), StandardCharsets.UTF_8).trim();
      return s;
    } catch (Exception e) {
      return "";
    }
  }

  private static String hostName() {
    try {
      return java.net.InetAddress.getLocalHost().getHostName();
    } catch (Exception e) {
      return "";
    }
  }

  private static String firstNonBlank(String... vals) {
    for (String v : vals) {
      if (v != null && !v.isBlank()) {
        return v.trim();
      }
    }
    return "";
  }

  private static String nvl(String s) {
    return s == null || s.isBlank() ? "NA" : s.trim();
  }
}
