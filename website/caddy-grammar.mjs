export default {
  "name": "caddyfile",
  "scopeName": "source.caddyfile",
  "patterns": [
    {
      "name": "comment.line.number-sign.caddyfile",
      "match": "#.*$"
    },
    {
      "name": "string.quoted.double.caddyfile",
      "begin": "\"",
      "end": "\""
    },
    {
      "name": "keyword.control.caddyfile",
      "match": "\\b(qwac|qwac_binding|hostname|directory|chain_files|key_file|trust_files|lifetime|acme|dir|email|reverse_proxy|file_server|root|tls|issuer|route|handle|firefox_compatibility)\\b"
    },
    {
      "name": "variable.other.caddyfile",
      "match": "\\{\\$[^}]+\\}"
    },
    {
      "name": "constant.numeric.caddyfile",
      "match": "\\b[0-9]+(?:h|s|m)?\\b"
    }
  ]
};
