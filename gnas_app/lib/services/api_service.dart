import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'package:http/http.dart' as http;
import 'package:shared_preferences/shared_preferences.dart';
import '../models/api_response.dart';
import '../models/system_info.dart';
import '../models/file_info.dart';
import '../models/media_item.dart';

class ApiService {
  static const String _hostKey = 'host_address';
  static const String _usernameKey = 'saved_username';
  static const String _passwordKey = 'saved_password';
  static const String _tokenKey = 'auth_token';
  static String _baseUrl = 'http://192.168.1.100:8082';
  static String _token = '';
  static const Duration _timeout = Duration(seconds: 20);
  static const Duration _duplicateTimeout = Duration(minutes: 10);
  static const Duration _archiveTimeout = Duration(minutes: 10);

  static final ApiService _instance = ApiService._internal();
  factory ApiService() => _instance;
  ApiService._internal();

  String get baseUrl => _baseUrl;

  /// Normalizes a host/URL string to a valid URL.
  /// - Adds http:// if missing
  /// - Wraps bare IPv6 addresses in square brackets
  static String normalizeUrl(String url) {
    url = url.trim();
    if (url.isEmpty) return url;

    if (!url.startsWith('http://') && !url.startsWith('https://')) {
      url = 'http://$url';
    }

    final authStart = url.indexOf('://') + 3;
    final authority = url.substring(authStart);

    // Detect bare IPv6 (contains >= 2 colons, no brackets)
    if (authority.contains(':') && !authority.contains('[')) {
      final colonCount = ':'.allMatches(authority).length;
      if (colonCount >= 2) {
        final lastColon = authority.lastIndexOf(':');
        final afterLastColon = authority.substring(lastColon + 1);

        String ipv6;
        String? portStr;

        if (RegExp(r'^\d+$').hasMatch(afterLastColon)) {
          ipv6 = authority.substring(0, lastColon);
          portStr = afterLastColon;
        } else {
          ipv6 = authority;
        }

        url = portStr != null
            ? '${url.substring(0, authStart)}[$ipv6]:$portStr'
            : '${url.substring(0, authStart)}[$ipv6]';
      }
    }

    return url;
  }

  Future<void> loadSaved() async {
    final prefs = await SharedPreferences.getInstance();
    _baseUrl = normalizeUrl(
      prefs.getString(_hostKey) ?? 'http://192.168.1.100:8082',
    );
    _token = prefs.getString(_tokenKey) ?? '';
  }

  Future<void> saveHost(String host) async {
    _baseUrl = normalizeUrl(host);
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_hostKey, _baseUrl);
  }

  Future<Map<String, String>> loadCredentials() async {
    final prefs = await SharedPreferences.getInstance();
    _token = prefs.getString(_tokenKey) ?? '';
    return {
      'username': prefs.getString(_usernameKey) ?? '',
      'password': prefs.getString(_passwordKey) ?? '',
      'host': prefs.getString(_hostKey) ?? 'http://192.168.1.100:8082',
    };
  }

  Future<void> saveCredentials(
    String username,
    String password,
    String host,
  ) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_usernameKey, username);
    await prefs.setString(_passwordKey, password);
    await prefs.setString(_hostKey, normalizeUrl(host));
    _baseUrl = normalizeUrl(host);
  }

  Future<void> clearSession({bool clearSavedPassword = false}) async {
    _token = '';
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove(_tokenKey);
    if (clearSavedPassword) {
      await prefs.remove(_passwordKey);
    }
  }

  Future<void> _saveToken(String token) async {
    _token = token;
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_tokenKey, token);
  }

  Map<String, String> get _authHeaders {
    return {if (_token.isNotEmpty) 'Authorization': 'Bearer $_token'};
  }

  Map<String, String> get _jsonHeaders => {
    ..._authHeaders,
    'Content-Type': 'application/json',
  };

  Future<ApiResponse<T>> _request<T>(
    Future<http.Response> Function() send,
    T Function(dynamic)? dataParser, {
    Duration timeout = _timeout,
  }) async {
    try {
      final r = await send().timeout(timeout);
      return _parseResponse(r, dataParser);
    } on SocketException {
      return ApiResponse(code: 1, message: '无法连接服务器');
    } on TimeoutException {
      return ApiResponse(code: 1, message: '请求超时');
    } on http.ClientException catch (e) {
      return ApiResponse(code: 1, message: '网络请求失败: ${e.message}');
    } catch (e) {
      return ApiResponse(code: 1, message: '请求失败: $e');
    }
  }

  ApiResponse<T> _parseResponse<T>(
    http.Response r,
    T Function(dynamic)? dataParser,
  ) {
    try {
      final decoded = jsonDecode(utf8.decode(r.bodyBytes));
      if (decoded is! Map<String, dynamic>) {
        return ApiResponse(code: r.statusCode, message: '服务器响应格式错误');
      }
      final res = ApiResponse.fromJson(decoded, dataParser);
      if (r.statusCode >= 400 && res.message == null) {
        return ApiResponse(
          code: res.code,
          message: '请求失败 (${r.statusCode})',
          data: res.data,
        );
      }
      return res;
    } catch (_) {
      return ApiResponse(code: r.statusCode, message: '服务器响应解析失败');
    }
  }

  Future<ApiResponse<void>> login(String username, String password) async {
    final res = await _request<Map<String, dynamic>>(
      () => http.post(
        Uri.parse('$_baseUrl/api/login'),
        headers: {'Content-Type': 'application/json'},
        body: jsonEncode({'username': username, 'password': password}),
      ),
      (d) => d as Map<String, dynamic>,
    );
    if (!res.isSuccess) {
      return ApiResponse(code: res.code, message: res.message);
    }
    final token = res.data?['token'] as String?;
    if (token == null || token.isEmpty) {
      return ApiResponse(code: 1, message: '登录响应缺少 token');
    }
    await _saveToken(token);
    return ApiResponse(code: res.code, message: res.message);
  }

  Future<ApiResponse<void>> logout() async {
    final res = await _request<void>(
      () => http.post(Uri.parse('$_baseUrl/api/logout'), headers: _jsonHeaders),
      (_) {},
    );
    await clearSession();
    return res;
  }

  Future<ApiResponse<void>> changePassword(String oldPwd, String newPwd) async {
    final res = await _request<void>(
      () => http.post(
        Uri.parse('$_baseUrl/api/change-password'),
        headers: _jsonHeaders,
        body: jsonEncode({'oldPassword': oldPwd, 'newPassword': newPwd}),
      ),
      (_) {},
    );
    if (res.isSuccess) {
      await clearSession(clearSavedPassword: true);
    }
    return res;
  }

  // -- Status --
  Future<ApiResponse<SystemStatus>> getStatus() async {
    return _request<SystemStatus>(
      () => http.get(Uri.parse('$_baseUrl/api/status'), headers: _authHeaders),
      (d) => SystemStatus.fromJson(d as Map<String, dynamic>),
    );
  }

  // -- System --
  Future<ApiResponse<SystemInfo>> getSystemInfo() async {
    return _request<SystemInfo>(
      () => http.get(Uri.parse('$_baseUrl/api/system'), headers: _authHeaders),
      (d) => SystemInfo.fromJson(d as Map<String, dynamic>),
    );
  }

  // -- Logs --
  Future<ApiResponse<List<String>>> getLogs() async {
    return _request<List<String>>(
      () => http.get(Uri.parse('$_baseUrl/api/logs'), headers: _authHeaders),
      (d) => (d as List<dynamic>).map((e) => e as String).toList(),
    );
  }

  Future<ApiResponse<void>> clearLogs() async {
    return _request<void>(
      () => http.post(
        Uri.parse('$_baseUrl/api/logs/clear'),
        headers: _jsonHeaders,
      ),
      (_) {},
    );
  }

  // -- Files --
  Future<ApiResponse<List<FileInfo>>> listFiles(String path) async {
    return _request<List<FileInfo>>(
      () => http.get(
        Uri.parse('$_baseUrl/api/files?path=${Uri.encodeComponent(path)}'),
        headers: _authHeaders,
      ),
      (d) => (d as List<dynamic>)
          .map((e) => FileInfo.fromJson(e as Map<String, dynamic>))
          .toList(),
    );
  }

  Future<ApiResponse<Map<String, dynamic>>> flattenFiles(String path) async {
    return _request<Map<String, dynamic>>(
      () => http.post(
        Uri.parse('$_baseUrl/api/files/flatten'),
        headers: _jsonHeaders,
        body: jsonEncode({'path': path}),
      ),
      (d) => d as Map<String, dynamic>,
    );
  }

  Future<ApiResponse<void>> uploadFile(String dirPath, File file) async {
    final req = http.MultipartRequest(
      'POST',
      Uri.parse(
        '$_baseUrl/api/files/upload?path=${Uri.encodeComponent(dirPath)}',
      ),
    );
    req.headers.addAll(_authHeaders);
    req.files.add(await http.MultipartFile.fromPath('file', file.path));
    return _request<void>(() async {
      final streamed = await req.send();
      return http.Response.fromStream(streamed);
    }, (_) {});
  }

  String getDownloadUrl(String path, {String? disposition}) {
    final params = {'path': path};
    if (disposition != null) params['disposition'] = disposition;
    if (_token.isNotEmpty) params['token'] = _token;
    final uri = Uri.parse(
      '$_baseUrl/api/files/download',
    ).replace(queryParameters: params);
    return uri.toString();
  }

  String getThumbUrl(String path) {
    final params = {'path': path};
    if (_token.isNotEmpty) params['token'] = _token;
    return Uri.parse(
      '$_baseUrl/api/files/thumb',
    ).replace(queryParameters: params).toString();
  }

  Future<ApiResponse<void>> createDir(String path) async {
    return _request<void>(
      () => http.post(
        Uri.parse('$_baseUrl/api/files/mkdir'),
        headers: _jsonHeaders,
        body: jsonEncode({'path': path}),
      ),
      (_) {},
    );
  }

  Future<ApiResponse<void>> renameFile(String oldPath, String newName) async {
    return _request<void>(
      () => http.post(
        Uri.parse('$_baseUrl/api/files/rename'),
        headers: _jsonHeaders,
        body: jsonEncode({'oldPath': oldPath, 'newName': newName}),
      ),
      (_) {},
    );
  }

  Future<ApiResponse<void>> deleteFile(String path) async {
    return _request<void>(
      () => http.post(
        Uri.parse('$_baseUrl/api/files/delete'),
        headers: _jsonHeaders,
        body: jsonEncode({'path': path}),
      ),
      (_) {},
    );
  }

  Future<ApiResponse<void>> batchDelete(List<String> paths) async {
    return _request<void>(
      () => http.post(
        Uri.parse('$_baseUrl/api/files/batch-delete'),
        headers: _jsonHeaders,
        body: jsonEncode({'paths': paths}),
      ),
      (_) {},
    );
  }

  // -- Gallery --
  Future<ApiResponse<List<MediaItem>>> getGallery() async {
    return _request<List<MediaItem>>(
      () => http.get(Uri.parse('$_baseUrl/api/gallery'), headers: _authHeaders),
      (d) => (d as List<dynamic>)
          .map((e) => MediaItem.fromJson(e as Map<String, dynamic>))
          .toList(),
    );
  }

  Future<http.Response> exportGallery() async {
    return http
        .get(Uri.parse('$_baseUrl/api/gallery/export'), headers: _authHeaders)
        .timeout(_archiveTimeout);
  }

  Future<ApiResponse<int>> importGallery(File file) async {
    try {
      final req = http.MultipartRequest(
        'POST',
        Uri.parse('$_baseUrl/api/gallery/import'),
      );
      req.headers.addAll(_authHeaders);
      req.files.add(await http.MultipartFile.fromPath('file', file.path));
      final streamed = await req.send().timeout(_archiveTimeout);
      final response = await http.Response.fromStream(streamed);
      return _parseResponse<int>(
        response,
        (d) => (d as Map<String, dynamic>)['imported'] as int? ?? 0,
      );
    } on SocketException {
      return ApiResponse(code: 1, message: 'Unable to connect to server');
    } on TimeoutException {
      return ApiResponse(code: 1, message: 'Request timed out');
    } catch (e) {
      return ApiResponse(code: 1, message: 'Import failed: $e');
    }
  }

  // -- Settings & AI --
  Future<ApiResponse<Map<String, dynamic>>> getSettings() async {
    return _request<Map<String, dynamic>>(
      () =>
          http.get(Uri.parse('$_baseUrl/api/settings'), headers: _authHeaders),
      (d) => d as Map<String, dynamic>,
    );
  }

  Future<ApiResponse<Map<String, dynamic>>> updateSettings({
    bool? aiEnabled,
    bool? sslEnabled,
    String? sslCertFile,
    String? sslKeyFile,
  }) async {
    final body = <String, dynamic>{};
    if (aiEnabled != null) body['ai_enabled'] = aiEnabled;
    if (sslEnabled != null) body['ssl_enabled'] = sslEnabled;
    if (sslCertFile != null) body['ssl_cert_file'] = sslCertFile;
    if (sslKeyFile != null) body['ssl_key_file'] = sslKeyFile;
    return _request<Map<String, dynamic>>(
      () => http.post(
        Uri.parse('$_baseUrl/api/settings/update'),
        headers: _jsonHeaders,
        body: jsonEncode(body),
      ),
      (d) => d as Map<String, dynamic>,
    );
  }

  Future<ApiResponse<List<MediaItem>>> searchGallery(String query) async {
    return _request<List<MediaItem>>(
      () => http.get(
        Uri.parse('$_baseUrl/api/search?q=${Uri.encodeComponent(query)}'),
        headers: _authHeaders,
      ),
      (d) => (d as List<dynamic>)
          .map((e) => MediaItem.fromJson(e as Map<String, dynamic>))
          .toList(),
    );
  }

  Future<ApiResponse<List<dynamic>>> getDuplicates() async {
    return _request<List<dynamic>>(
      () => http.get(
        Uri.parse('$_baseUrl/api/gallery/duplicates'),
        headers: _authHeaders,
      ),
      (d) => d as List<dynamic>,
      timeout: _duplicateTimeout,
    );
  }
}
