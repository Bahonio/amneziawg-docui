#!/bin/sh
# Installer helpers, sourced only after the release bundle is verified.
# Variables for CLI choices are initialized by install.sh.
# shellcheck disable=SC2154

web_env_value() {
    [ -f "$1" ] || return 0
    awk -v key="$2" '{
            line=$0
            sub(/\r$/, "", line)
            sub(/^[ \t]*/, "", line)
            sub(/^export[ \t]+/, "", line)
            if (substr(line,1,length(key)) != key) next
            rest=substr(line,length(key)+1)
            if (rest !~ /^[ \t]*=/) next
            sub(/^[ \t]*=[ \t]*/, "", rest)
            value=rest
        }
        END {
            sub(/[ \t]*$/, "", value)
            if (value ~ /^"[^"]*"([ \t]*#.*)?$/ || value ~ /^\047[^\047]*\047([ \t]*#.*)?$/) {
                quote=substr(value,1,1)
                value=substr(value,2)
                value=substr(value,1,index(value,quote)-1)
            } else {
                sub(/[ \t]+#.*/, "", value)
                sub(/[ \t]*$/, "", value)
            }
            print value
        }' "$1"
}

web_valid_port() {
    case "$1" in ''|*[!0-9]*) return 1 ;; esac
    [ "${#1}" -le 5 ] && [ "$1" -ge 1 ] && [ "$1" -le 65535 ]
}

web_valid_ip() {
    case "$1" in ''|*[!0-9a-fA-F:.]*) return 1 ;; esac
    awk -v ip="$1" '
        function ipv4(s, a, n, i) {
            n=split(s,a,".")
            if (n != 4) return 0
            for (i=1;i<=n;i++)
                if (a[i] !~ /^[0-9]+$/ || length(a[i])>3 || a[i]+0>255 ||
                    (length(a[i])>1 && substr(a[i],1,1)=="0")) return 0
            return 1
        }
        function groups(s, a, n, i, count) {
            if (s=="") return 0
            n=split(s,a,":"); count=0
            for (i=1;i<=n;i++) {
                if (a[i] ~ /\./ && i==n && ipv4(a[i])) count+=2
                else if (a[i] ~ /^[0-9a-fA-F]+$/ && length(a[i])<=4) count++
                else return -100
            }
            return count
        }
        BEGIN {
            if (index(ip,":")==0) exit !ipv4(ip)
            n=split(ip,parts,"::")
            if (n>2 || ip ~ /[^0-9a-fA-F:.]/) exit 1
            left=groups(parts[1]); right=groups(parts[2])
            if (left<0 || right<0 || (n==2 && parts[1] ~ /\./)) exit 1
            if (n==1) exit !(left==8)
            exit !(left+right<8)
        }'
}

write_web_access() {
    set_env_value "$1" WEB_UI_BIND_ADDRESS "$bind_address"
    set_env_value "$1" WEB_UI_PORT "$web_port"
    set_env_value "$1" WEB_UI_URL "$panel_url"
}

web_valid_url() {
    case "$1" in http://*) url_authority=${1#http://} ;; https://*) url_authority=${1#https://} ;; *) return 1 ;; esac
    url_authority=${url_authority%/}
    case "$url_authority" in ''|*[!A-Za-z0-9.:[\]-]*) return 1 ;; esac
    url_port=""
    case "$url_authority" in
        \[*\]*)
            url_host=${url_authority#\[}; url_host=${url_host%%\]*}
            web_valid_ip "$url_host" || return 1
            case "$url_host" in *:*) ;; *) return 1 ;; esac
            url_suffix=${url_authority#*\]}
            case "$url_suffix" in '') ;; :*) url_port=${url_suffix#:}; web_valid_port "$url_port" || return 1 ;; *) return 1 ;; esac
            ;;
        *)
            url_host=${url_authority%%:*}
            case "$url_authority" in *:*) url_port=${url_authority#*:}; web_valid_port "$url_port" || return 1 ;; esac
            case "$url_host" in
                *[!0-9.]*)
                    printf '%s\n' "$url_host" | awk '
                        length($0)>253 { exit 1 }
                        { n=split($0,a,"."); for (i=1;i<=n;i++)
                            if (length(a[i])<1 || length(a[i])>63 || a[i] !~ /^[A-Za-z0-9-]+$/ || a[i] ~ /^-/ || a[i] ~ /-$/) exit 1 }' || return 1
                    ;;
                *) web_valid_ip "$url_host" || return 1 ;;
            esac
            ;;
    esac
    case "$url_host" in 0.0.0.0|::) return 1 ;; esac
}

web_check_host_address() {
    case "$1" in 127.0.0.1|::1|0.0.0.0|::) return 0 ;; esac
    command -v ip >/dev/null 2>&1 || {
        echo 'The ip command is required to check the selected host address.' >&2; return 1;
    }
    assigned_address=$(ip -o addr show to "$1" 2>/dev/null) || return 1
    [ -n "$assigned_address" ] || {
        echo "Address $1 is not assigned to this host. Use the server address, not a client address." >&2
        return 1
    }
}

web_prompt() {
    printf '%s [%s]: ' "$1" "$2" >&5
    IFS= read -r web_reply <&4 || { echo 'Access configuration cancelled: terminal input ended.' >&2; return 1; }
    web_reply=${web_reply:-$2}
}

web_access_dialogue() {
    printf '\nHow will you access the panel?\n1) SSH tunnel (default)\n2) VPN / local network\n3) Domain through a reverse proxy\n4) All interfaces\n' >&5
    while :; do
        web_prompt 'Choose 1-4' 1 || return 1
        case "$web_reply" in 1) access=ssh; break ;; 2) access=vpn; break ;; 3) access=proxy; break ;; 4) access=all; break ;; esac
        printf 'Choose a number from 1 to 4.\n' >&5
    done
    panel_url=""
    case "$access" in
        ssh|proxy) bind_address=127.0.0.1 ;;
        all) bind_address=0.0.0.0 ;;
        vpn)
            command -v ip >/dev/null 2>&1 || { echo 'The ip command is required for VPN / LAN access.' >&2; return 1; }
            host_addresses=$(ip -o addr show scope global | awk '{ sub(/\/.*/, "", $4); print $4 }')
            printf 'Addresses on this server:\n' >&5
            printf '%s\n' "$host_addresses" | awk 'NF { printf "%d) %s\n", ++n, $0 }' >&5
            case "$bind_address" in
                127.*|::1|0.0.0.0|::) bind_address=$(printf '%s\n' "$host_addresses" | sed -n '1p') ;;
            esac
            while :; do
                web_prompt 'Select an address number or enter a server IP' "$bind_address" || return 1
                case "$web_reply" in
                    *[!0-9]*|'') bind_address=$web_reply ;;
                    *) bind_address=$(printf '%s\n' "$host_addresses" | awk -v n="$web_reply" 'NR==n { print; exit }') ;;
                esac
                case "$bind_address" in 127.*|::1|0.0.0.0|::) printf 'Choose a VPN / LAN address on this server.\n' >&5; continue ;; esac
                if web_valid_ip "$bind_address" && web_check_host_address "$bind_address"; then break; fi
                printf 'Enter a valid address assigned to this server.\n' >&5
            done
            ;;
    esac
    while :; do
        web_prompt 'Panel TCP port' "$web_port" || return 1
        if web_valid_port "$web_reply"; then web_port=$web_reply; break; fi
        printf 'Enter a port from 1 to 65535.\n' >&5
    done
    if [ "$access" = proxy ]; then
        printf 'Configure DNS and your HTTPS reverse proxy separately. Preserve the original Host header.\n' >&5
        printf 'The proxy on this host must forward to http://127.0.0.1:%s.\n' "$web_port" >&5
        while :; do
            web_prompt 'Panel URL (http:// or https://, without a path)' '' || return 1
            if web_valid_url "$web_reply"; then panel_url=$web_reply; break; fi
            printf 'Enter a URL such as https://panel.example.com.\n' >&5
        done
    fi
}

configure_web_access() {
    bind_address=$(web_env_value "$1" WEB_UI_BIND_ADDRESS); bind_address=${bind_address:-127.0.0.1}
    web_port=$(web_env_value "$1" WEB_UI_PORT); web_port=${web_port:-54845}
    panel_url=$(web_env_value "$1" WEB_UI_URL)

    case "$access" in
        '') ;;
        ssh|proxy) bind_address=127.0.0.1; panel_url="" ;;
        vpn) panel_url="" ;;
        all) bind_address=0.0.0.0; panel_url="" ;;
        *) echo 'Invalid --access: use ssh, vpn, proxy or all.' >&2; return 1 ;;
    esac
    if [ -n "$requested_bind" ]; then bind_address=$requested_bind; panel_url=""; fi
    [ -z "$requested_port" ] || web_port=$requested_port
    if [ "$url_was_requested" = yes ]; then panel_url=$requested_url; fi

    access_prompt=no
    if [ "$configure_access" = yes ]; then
        [ "$non_interactive" = no ] || { echo '--configure-access requires a terminal.' >&2; return 1; }
        access_prompt=yes
    elif [ ! -f "$1" ] && [ "$non_interactive" = no ] && [ "$access_flags" = no ]; then
        access_prompt=auto
    fi
    if [ "$access_prompt" != no ]; then
        if ( : </dev/tty ) 2>/dev/null; then
            exec 4</dev/tty 5>/dev/tty
            web_access_dialogue || return 1
            exec 4<&- 5>&-
        elif [ "$access_prompt" = yes ]; then
            echo 'No terminal available. Use --access, --bind-address, --web-port and --panel-url.' >&2
            return 1
        fi
    fi

    web_valid_ip "$bind_address" || { echo "Invalid bind IP: $bind_address. Hostnames belong in --panel-url." >&2; return 1; }
    web_valid_port "$web_port" || { echo "Invalid panel port: $web_port" >&2; return 1; }
    # Avoid ambiguous leading-zero ports in URLs and Compose.
    web_port=$(printf '%s\n' "$web_port" | awk '{ print $0+0 }')
    if [ -n "$panel_url" ]; then
        web_valid_url "$panel_url" || { echo 'Invalid panel URL: use http(s)://host[:port] without credentials or a path.' >&2; return 1; }
    fi
    case "$access" in
        ssh|proxy)
            case "$bind_address" in 127.0.0.1|::1) ;; *) echo 'SSH / proxy access requires a loopback bind address.' >&2; return 1 ;; esac
            ;;
        vpn)
            case "$bind_address" in 127.*|::1|0.0.0.0|::) echo 'VPN / LAN access requires a specific server IP.' >&2; return 1 ;; esac
            ;;
        all)
            case "$bind_address" in 0.0.0.0|::) ;; *) echo 'All-interface access requires 0.0.0.0 or ::.' >&2; return 1 ;; esac
            ;;
    esac
    [ "$access" != proxy ] || [ -n "$panel_url" ] || { echo '--access proxy requires --panel-url.' >&2; return 1; }
    # An unchanged binding may be temporarily absent during an update/reboot.
    if [ -n "$requested_bind" ] || [ -n "$access" ]; then
        web_check_host_address "$bind_address" || return 1
    fi
    echo "Panel host binding: $bind_address:$web_port"
    case "$bind_address" in
        0.0.0.0|::) echo 'The panel will be published on all host interfaces. Restrict access with your firewall or reverse proxy.' ;;
    esac
}

print_web_access() {
    case "$bind_address" in *:*) url_bind="[$bind_address]" ;; *) url_bind=$bind_address ;; esac
    echo "Panel host binding: $url_bind:$web_port"
    if [ -n "$panel_url" ]; then
        echo "Panel URL: $panel_url"
        echo 'This URL requires your DNS / proxy configuration; the installer does not configure it.'
    else
        case "$bind_address" in
            0.0.0.0|::) echo "Panel URL: http://YOUR_SERVER_IP:$web_port (use a reachable server IP or DNS name)" ;;
            *) echo "Panel URL: http://$url_bind:$web_port" ;;
        esac
    fi
    case "$bind_address" in
        127.0.0.1|::1)
            if [ -n "$panel_url" ]; then
                echo "Reverse proxy upstream on this host: http://$url_bind:$web_port; preserve the original Host header."
            else
                echo "Open it locally with: ssh -L $web_port:$url_bind:$web_port root@YOUR_SERVER"
            fi
            ;;
        *) echo 'Access depends on your network routing and firewall; neither is changed by this setting.' ;;
    esac
}

set_env_value() {
    env_file=$1
    env_key=$2
    env_value=$3
    env_tmp=$(mktemp "$env_file.tmp.XXXXXX")
    awk -v key="$env_key" -v value="$env_value" '
        BEGIN { found=0 }
        index($0, key "=") == 1 {
            if (!found) print key "=" value
            found=1
            next
        }
        { print }
        END { if (!found) print key "=" value }
    ' "$env_file" > "$env_tmp"
    chmod 0600 "$env_tmp"
    mv -f "$env_tmp" "$env_file"
}
