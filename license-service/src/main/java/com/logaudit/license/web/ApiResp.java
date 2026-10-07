package com.logaudit.license.web;

/** 与平台 Go 后端一致的统一响应结构：{code, message, data} */
public record ApiResp<T>(int code, String message, T data) {
  public static <T> ApiResp<T> ok(T data) {
    return new ApiResp<>(0, "ok", data);
  }

  public static <T> ApiResp<T> fail(String message) {
    return new ApiResp<>(1, message, null);
  }
}
