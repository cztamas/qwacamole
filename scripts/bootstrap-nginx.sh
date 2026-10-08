#!/bin/sh
# Development toolchain only; production operators install supported packages.
set -eu
cd "$(dirname "$0")/.."
mkdir -p .tools/nginx-build .local/evidence/agent-nginx
if [ ! -x .tools/certbot-venv/bin/certbot ]; then
 python3 -m venv .tools/certbot-venv
 .tools/certbot-venv/bin/pip install 'certbot==5.8.0' > .local/evidence/agent-nginx/certbot-install.log 2>&1
fi
if [ ! -x .tools/nginx/sbin/nginx ]; then
 case "$(uname -s)" in
 Darwin)
  openssl=$(brew --prefix openssl@3)
  pcre=$(brew --prefix pcre2)
  curl -fsSL https://nginx.org/download/nginx-1.30.5.tar.gz -o .tools/nginx-build/nginx-1.30.5.tar.gz
  echo '6c20565aa2325cb82216ae804f4a4ff1875179014759a381c42ddc8e11c4906d  .tools/nginx-build/nginx-1.30.5.tar.gz' | shasum -a 256 -c -
  tar -xzf .tools/nginx-build/nginx-1.30.5.tar.gz -C .tools/nginx-build
  root=$PWD
  cd .tools/nginx-build/nginx-1.30.5
  ./configure --prefix="$root/.tools/nginx" --without-http_gzip_module --with-http_ssl_module --with-http_v2_module --with-http_auth_request_module "--with-cc-opt=-I$openssl/include -I$pcre/include" "--with-ld-opt=-L$openssl/lib -L$pcre/lib" > "$root/.local/evidence/agent-nginx/nginx-build.log" 2>&1
  make -j4 >> "$root/.local/evidence/agent-nginx/nginx-build.log" 2>&1
  make install >> "$root/.local/evidence/agent-nginx/nginx-build.log" 2>&1
  cd "$root"
  ;;
 *) echo 'Install Nginx with http_ssl/http_auth_request modules and set NGINX=/absolute/path; source bootstrap currently supports macOS.' >&2; exit 1;;
 esac
fi
.tools/nginx/sbin/nginx -V
.tools/certbot-venv/bin/certbot --version
