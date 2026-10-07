package com.logaudit.license.data;

import jakarta.annotation.PostConstruct;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.stereotype.Repository;

import java.time.LocalDateTime;
import java.util.List;
import java.util.Map;

/**
 * 授权数据访问（MySQL audit_meta）：
 *  - license_active   ：当前生效授权（单条，id=1）
 *  - license_issue_log：授权签发历史（等保三级：授权操作留痕）
 */
@Repository
public class LicenseRepository {

  private final JdbcTemplate jdbc;

  public LicenseRepository(JdbcTemplate jdbc) {
    this.jdbc = jdbc;
  }

  @PostConstruct
  public void init() {
    RuntimeException last = null;
    for (int i = 0; i < 30; i++) {
      try {
        jdbc.execute("""
            CREATE TABLE IF NOT EXISTS license_active (
              id           INT PRIMARY KEY,
              hardware_id  VARCHAR(128) NOT NULL,
              license_code LONGTEXT     NOT NULL,
              permanent    TINYINT      NOT NULL DEFAULT 0,
              expire_date  VARCHAR(32)  NOT NULL DEFAULT '',
              issued_at    BIGINT       NOT NULL DEFAULT 0,
              activated_by VARCHAR(64)  NOT NULL DEFAULT '',
              activated_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP
            ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4""");
        jdbc.execute("""
            CREATE TABLE IF NOT EXISTS license_issue_log (
              id          BIGINT AUTO_INCREMENT PRIMARY KEY,
              hardware_id VARCHAR(128) NOT NULL,
              license_code LONGTEXT    NOT NULL,
              permanent   TINYINT      NOT NULL DEFAULT 0,
              expire_date VARCHAR(32)  NOT NULL DEFAULT '',
              days        INT          NOT NULL DEFAULT 0,
              customer    VARCHAR(128) NOT NULL DEFAULT '',
              note        VARCHAR(255) NOT NULL DEFAULT '',
              operator    VARCHAR(64)  NOT NULL DEFAULT '',
              issued_at   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
              KEY idx_hw (hardware_id),
              KEY idx_time (issued_at)
            ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4""");
        return;
      } catch (RuntimeException e) {
        last = e;
        try {
          Thread.sleep(2000);
        } catch (InterruptedException ie) {
          Thread.currentThread().interrupt();
          break;
        }
      }
    }
    throw new IllegalStateException("授权表初始化失败（MySQL 不可用）", last);
  }

  /** 写入/覆盖当前生效授权 */
  public void saveActive(String hardwareId, String code, boolean permanent,
                         String expireDate, long issuedAt, String operator) {
    jdbc.update("""
            INSERT INTO license_active
              (id, hardware_id, license_code, permanent, expire_date, issued_at, activated_by, activated_at)
            VALUES (1, ?, ?, ?, ?, ?, ?, ?)
            ON DUPLICATE KEY UPDATE
              hardware_id = VALUES(hardware_id),
              license_code = VALUES(license_code),
              permanent = VALUES(permanent),
              expire_date = VALUES(expire_date),
              issued_at = VALUES(issued_at),
              activated_by = VALUES(activated_by),
              activated_at = VALUES(activated_at)""",
        hardwareId, code, permanent ? 1 : 0, expireDate == null ? "" : expireDate,
        issuedAt, operator == null ? "" : operator, LocalDateTime.now());
  }

  public Map<String, Object> loadActive() {
    List<Map<String, Object>> rows = jdbc.queryForList(
        "SELECT hardware_id, license_code, permanent, expire_date, issued_at, activated_by, activated_at "
            + "FROM license_active WHERE id = 1");
    return rows.isEmpty() ? null : rows.get(0);
  }

  public void clearActive() {
    jdbc.update("DELETE FROM license_active WHERE id = 1");
  }

  /** 签发留痕 */
  public long insertRecord(String hardwareId, String code, boolean permanent,
                           String expireDate, int days, String customer, String note, String operator) {
    jdbc.update("""
            INSERT INTO license_issue_log
              (hardware_id, license_code, permanent, expire_date, days, customer, note, operator, issued_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)""",
        hardwareId, code, permanent ? 1 : 0, expireDate == null ? "" : expireDate, days,
        customer == null ? "" : customer, note == null ? "" : note,
        operator == null ? "" : operator, LocalDateTime.now());
    Long id = jdbc.queryForObject("SELECT LAST_INSERT_ID()", Long.class);
    return id == null ? 0L : id;
  }

  public List<Map<String, Object>> listRecords(int limit) {
    return jdbc.queryForList("""
        SELECT id, hardware_id, license_code, permanent, expire_date, days,
               customer, note, operator, issued_at
        FROM license_issue_log ORDER BY id DESC LIMIT ?""", limit);
  }

  public Map<String, Object> findRecord(long id) {
    List<Map<String, Object>> rows = jdbc.queryForList("""
        SELECT id, hardware_id, license_code, permanent, expire_date, days,
               customer, note, operator, issued_at
        FROM license_issue_log WHERE id = ?""", id);
    return rows.isEmpty() ? null : rows.get(0);
  }
}
