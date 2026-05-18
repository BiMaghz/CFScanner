#!/bin/bash
#===============================================================================
# FILE: cfScanner.sh
# USAGE: ./cfScanner.sh [Arguments]
# DESCRIPTION: Scan CDN edge IP addresses for Cloudflare/Fastly/Akamai.
#              Cloudflare behavior is kept as the default.
# REQUIREMENTS: getopt, jq, git, tput, bc, curl, parallel, shuf
#===============================================================================

export TOP_PID=$$

fncLongIntToStr() {
  local IFS=.
  local num quad ip e
  num=$1
  for e in 3 2 1; do
    (( quad = 256 ** e ))
    (( ip[3-e] = num / quad ))
    (( num = num % quad ))
  done
  ip[3]=$num
  echo "${ip[*]}"
}

fncIpToLongInt() {
  local IFS=.
  local ip num e
  # shellcheck disable=SC2206
  ip=($1)
  num=0
  for e in 3 2 1; do
    (( num += ip[3-e] * 256 ** e ))
  done
  (( num += ip[3] ))
  echo "$num"
}

fncSubnetToIP() {
  # shellcheck disable=SC2206
  local network=(${1//\// })
  # shellcheck disable=SC2206
  local iparr=(${network[0]//./ })
  local mask=32
  [[ $((${#network[@]})) -gt 1 ]] && mask=${network[1]}
  local maskarr ipList i j k l bytes

  if [[ ${mask} = '\.' ]]; then
    # shellcheck disable=SC2206
    maskarr=(${mask//./ })
  else
    if [[ $((mask)) -lt 8 ]]; then
      maskarr=($((256-2**(8-mask))) 0 0 0)
    elif [[ $((mask)) -lt 16 ]]; then
      maskarr=(255 $((256-2**(16-mask))) 0 0)
    elif [[ $((mask)) -lt 24 ]]; then
      maskarr=(255 255 $((256-2**(24-mask))) 0)
    elif [[ $((mask)) -lt 32 ]]; then
      maskarr=(255 255 255 $((256-2**(32-mask))))
    elif [[ ${mask} == 32 ]]; then
      maskarr=(255 255 255 255)
    fi
  fi

  [[ ${maskarr[2]} == 255 ]] && maskarr[1]=255
  [[ ${maskarr[1]} == 255 ]] && maskarr[0]=255

  if [[ "$randomNumber" != "NULL" ]]; then
    bytes=(0 0 0 0)
    for i in $(seq 0 $((255-maskarr[0]))); do
      bytes[0]="$(( i+(iparr[0] & maskarr[0]) ))"
      for j in $(seq 0 $((255-maskarr[1]))); do
        bytes[1]="$(( j+(iparr[1] & maskarr[1]) ))"
        for k in $(seq 0 $((255-maskarr[2]))); do
          bytes[2]="$(( k+(iparr[2] & maskarr[2]) ))"
          for l in $(seq 1 $((255-maskarr[3]))); do
            bytes[3]="$(( l+(iparr[3] & maskarr[3]) ))"
            ipList+=("$(printf "%d.%d.%d.%d" "${bytes[@]}")")
          done
        done
      done
    done

    if [[ "$osVersion" == "Linux" ]]; then
      mapfile -t ipList < <(shuf -e "${ipList[@]}")
      mapfile -t ipList < <(shuf -e "${ipList[@]:0:$randomNumber}")
    elif [[ "$osVersion" == "Mac" ]]; then
      # shellcheck disable=SC2207
      ipList=($(printf '%s\n' "${ipList[@]}" | shuf))
      # shellcheck disable=SC2207
      ipList=($(printf '%s\n' "${ipList[@]:0:$randomNumber}" | shuf))
    else
      echo "OS not supported only Linux or Mac"
      exit 1
    fi

    for i in "${ipList[@]}"; do echo "$i"; done
  else
    bytes=(0 0 0 0)
    for i in $(seq 0 $((255-maskarr[0]))); do
      bytes[0]="$(( i+(iparr[0] & maskarr[0]) ))"
      for j in $(seq 0 $((255-maskarr[1]))); do
        bytes[1]="$(( j+(iparr[1] & maskarr[1]) ))"
        for k in $(seq 0 $((255-maskarr[2]))); do
          bytes[2]="$(( k+(iparr[2] & maskarr[2]) ))"
          for l in $(seq 1 $((255-maskarr[3]))); do
            bytes[3]="$(( l+(iparr[3] & maskarr[3]) ))"
            printf "%d.%d.%d.%d\n" "${bytes[@]}"
          done
        done
      done
    done
  fi
}

fncShowProgress() {
  local barCharDone="=" barCharTodo=" " barSplitter='>' barPercentageScale=2
  local current="$1" total="$2" barSize percent done todo doneSubBar todoSubBar spacesSubBar
  barSize="$(($(tput cols)-70))"
  percent=$(bc <<< "scale=$barPercentageScale; 100 * $current / $total")
  done=$(bc <<< "scale=0; $barSize * $percent / 100")
  todo=$(bc <<< "scale=0; $barSize - $done")
  doneSubBar=$(printf "%${done}s" | tr " " "${barCharDone}")
  todoSubBar=$(printf "%${todo}s" | tr " " "${barCharTodo} - 1")
  spacesSubBar=$(printf "%${todo}s" | tr " " " ")
  progressBar="| Progress bar of main IPs: [${doneSubBar}${barSplitter}${todoSubBar}] ${percent}%${spacesSubBar}"
}

fncGetTimeoutCommand() {
  if command -v timeout >/dev/null 2>&1; then
    echo "timeout"
  elif command -v gtimeout >/dev/null 2>&1; then
    echo "gtimeout"
  else
    echo >&2 "I require 'timeout' command but it's not installed. Please install 'timeout' or 'gtimeout' and try again."
    exit 1
  fi
}

fncCurlMs() {
  local timeoutCommand="$1"
  shift
  local result
  result=$($timeoutCommand 2 curl "$@" 2>/dev/null | grep "TIME" | tail -n 1 | awk '{print $2}' | xargs -I {} echo "{} * 1000 /1" | bc)
  [[ "$result" ]] && echo "$result" || echo "0"
}

fncCheckIPList() {
  local ipList resultFile scriptDir configId configHost configPort configPath fileSize osVersion clientCommand tryCount downThreshold upThreshold downloadOrUpload vpnOrNot quickOrNot provider testHost downloadPath uploadPath
  ipList="${1}"
  resultFile="${3}"
  scriptDir="${4}"
  configId="${5}"
  configHost="${6}"
  configPort="${7}"
  configPath="${8}"
  fileSize="${9}"
  osVersion="${10}"
  clientCommand="${11}"
  tryCount="${12}"
  downThreshold="${13}"
  upThreshold="${14}"
  downloadOrUpload="${15}"
  vpnOrNot="${16}"
  quickOrNot="${17}"
  provider="${18}"
  testHost="${19}"
  downloadPath="${20}"
  uploadPath="${21}"

  local binDir="$scriptDir/../bin"
  local tempConfigDir="$scriptDir/tempConfig"
  local uploadFile="$tempConfigDir/upload_file"
  local timeoutCommand domainFronting ip downOK upOK downTotalTime upTotalTime downAvgStr upAvgStr downSuccessedCount upSuccessedCount downTimeMil upTimeMil result downRealTime upRealTime

  timeoutCommand=$(fncGetTimeoutCommand)
  configPath=$(echo "$configPath" | sed 's/\//\\\//g')

  for ip in ${ipList}; do
    if [[ "$downloadOrUpload" == "BOTH" ]]; then
      downOK="NO"; upOK="NO"
    elif [[ "$downloadOrUpload" == "UP" ]]; then
      downOK="YES"; upOK="NO"
    elif [[ "$downloadOrUpload" == "DOWN" ]]; then
      downOK="NO"; upOK="YES"
    fi
    
    domainFronting="NULL"
    if [[ "$quickOrNot" == "NO" ]]; then
      echo $timeoutCommand 1 curl -k -s --tlsv1.2 -H "Host: $testHost" --resolve "$testHost:443:$ip" "https://$testHost${downloadPath}10" 2>/dev/null
      domainFronting=$($timeoutCommand 1 curl -k -s --tlsv1.2 -H "Host: $testHost" --resolve "$testHost:443:$ip" "https://$testHost${downloadPath}10" 2>/dev/null)
    fi

    if [[ "$domainFronting" != "NULL" && "$quickOrNot" == "NO" ]]; then
      echo -e "${RED}FAILED - DF${NC} $ip"
      continue
    fi

    if [[ "$vpnOrNot" == "YES" ]]; then
      local mainDomain randomUUID configServerName ipConfigFile ipO1 ipO2 ipO3 ipO4 port pid
      mainDomain=$(echo "$configHost" | awk -F '.' '{ print $(NF-1)"."$NF }')
      if [[ "$osVersion" == "Linux" ]]; then
        randomUUID=$(cat /proc/sys/kernel/random/uuid)
      elif [[ "$osVersion" == "Mac" ]]; then
        randomUUID=$(uuidgen | tr '[:upper:]' '[:lower:]')
      else
        echo "OS not supported only Linux or Mac"
        exit 1
      fi
      configServerName="$randomUUID.$mainDomain"
      ipConfigFile="$tempConfigDir/config.json.$ip.json"
      cp "$scriptDir/config.json.temp" "$ipConfigFile"
      ipO1=$(echo "$ip" | awk -F '.' '{print $1}')
      ipO2=$(echo "$ip" | awk -F '.' '{print $2}')
      ipO3=$(echo "$ip" | awk -F '.' '{print $3}')
      ipO4=$(echo "$ip" | awk -F '.' '{print $4}')
      port=$((ipO1 + ipO2 + ipO3 + ipO4))

      if [[ "$osVersion" == "Mac" ]]; then
        sed -i "" "s/IP.IP.IP.IP/$ip/g" "$ipConfigFile"
        sed -i "" "s/PORTPORT/3$port/g" "$ipConfigFile"
        sed -i "" "s/IDID/$configId/g" "$ipConfigFile"
        sed -i "" "s/HOSTHOST/$configHost/g" "$ipConfigFile"
        sed -i "" "s/CFPORTCFPORT/$configPort/g" "$ipConfigFile"
        sed -i "" "s/ENDPOINTENDPOINT/$configPath/g" "$ipConfigFile"
        sed -i "" "s/RANDOMHOST/$configServerName/g" "$ipConfigFile"
      else
        sed -i "s/IP.IP.IP.IP/$ip/g" "$ipConfigFile"
        sed -i "s/PORTPORT/3$port/g" "$ipConfigFile"
        sed -i "s/IDID/$configId/g" "$ipConfigFile"
        sed -i "s/HOSTHOST/$configHost/g" "$ipConfigFile"
        sed -i "s/CFPORTCFPORT/$configPort/g" "$ipConfigFile"
        sed -i "s/ENDPOINTENDPOINT/$configPath/g" "$ipConfigFile"
        sed -i "s/RANDOMHOST/$configServerName/g" "$ipConfigFile"
      fi

      pid=$(ps aux | grep "config.json.$ip" | grep -v grep | awk '{ print $2 }')
      [[ "$pid" ]] && kill -9 "$pid" >/dev/null 2>&1
      nohup "$binDir/$clientCommand" -c "$ipConfigFile" >/dev/null &
      sleep 2
    fi

    downTotalTime=0; upTotalTime=0; downAvgStr=""; upAvgStr=""; downSuccessedCount=0; upSuccessedCount=0
    for _ in $(seq 1 "$tryCount"); do
      downTimeMil=0; upTimeMil=0
      if [[ "$downloadOrUpload" == "DOWN" || "$downloadOrUpload" == "BOTH" ]]; then
        if [[ "$vpnOrNot" == "YES" ]]; then
          downTimeMil=$(fncCurlMs "$timeoutCommand" -x "socks5://127.0.0.1:3$port" -s -w "TIME: %{time_total}\n" --resolve "$testHost:443:$ip" "https://$testHost${downloadPath}$fileSize" --output /dev/null)
        else
          downTimeMil=$(fncCurlMs "$timeoutCommand" -s -w "TIME: %{time_total}\n" -H "Host: $testHost" --resolve "$testHost:443:$ip" "https://$testHost${downloadPath}$fileSize" --output /dev/null)
        fi
        if [[ $downTimeMil -gt 100 ]]; then downSuccessedCount=$((downSuccessedCount+1)); else downTimeMil=0; fi
      fi

      if [[ "$downloadOrUpload" == "UP" || "$downloadOrUpload" == "BOTH" ]]; then
        if [[ "$vpnOrNot" == "YES" ]]; then
          result=$(fncCurlMs "$timeoutCommand" -x "socks5://127.0.0.1:3$port" -s -w "\nTIME: %{time_total}\n" --resolve "$testHost:443:$ip" --data "@$uploadFile" "https://$testHost${uploadPath}")
        else
          result=$(fncCurlMs "$timeoutCommand" -s -w "\nTIME: %{time_total}\n" -H "Host: $testHost" --resolve "$testHost:443:$ip" --data "@$uploadFile" "https://$testHost${uploadPath}")
        fi
        upTimeMil="$result"
        if [[ $upTimeMil -gt 100 ]]; then upSuccessedCount=$((upSuccessedCount+1)); else upTimeMil=0; fi
      fi

      downTotalTime=$((downTotalTime+downTimeMil))
      upTotalTime=$((upTotalTime+upTimeMil))
      downAvgStr="$downAvgStr $downTimeMil"
      upAvgStr="$upAvgStr $upTimeMil"
    done

    if [[ $downSuccessedCount -ge $downThreshold && "$downloadOrUpload" != "UP" ]]; then downOK="YES"; downRealTime=$((downTotalTime/downSuccessedCount)); else downRealTime=0; fi
    if [[ $upSuccessedCount -ge $upThreshold && "$downloadOrUpload" != "DOWN" ]]; then upOK="YES"; upRealTime=$((upTotalTime/upSuccessedCount)); else upRealTime=0; fi

    if [[ "$vpnOrNot" == "YES" ]]; then
      pid=$(ps aux | grep "config.json.$ip" | grep -v grep | awk '{ print $2 }')
      [[ "$pid" ]] && kill -9 "$pid" >/dev/null 2>&1
    fi

    if [[ "$downOK" == "YES" && "$upOK" == "YES" ]]; then
      if [[ "$downRealTime" && $downRealTime -gt 100 ]] || [[ "$upRealTime" && $upRealTime -gt 100 ]]; then
        echo -e "${GREEN}OK${NC} $ip ${BLUE}DOWN: Avg $downRealTime $downAvgStr ${ORANGE}UP: Avg $upRealTime, $upAvgStr${NC}"
        [[ "$downRealTime" && $downRealTime -gt 100 ]] && echo "$downRealTime, $downAvgStr DOWN FOR IP $ip" >> "$resultFile"
        [[ "$upRealTime" && $upRealTime -gt 100 ]] && echo "$upRealTime, $upAvgStr UP FOR IP $ip" >> "$resultFile"
      else
        echo -e "${RED}FAILED${NC} $ip"
      fi
    else
      echo -e "${RED}FAILED${NC} $ip"
    fi
  done
}
export -f fncCheckIPList fncGetTimeoutCommand fncCurlMs

fncCheckDpnd() {
  local osVersion="NULL"
  if [[ "$(uname)" == "Linux" ]]; then
    command -v jq >/dev/null 2>&1 || { echo >&2 "I require 'jq' but it's not installed. Please install it and try again."; kill -s 1 "$TOP_PID"; }
    command -v parallel >/dev/null 2>&1 || { echo >&2 "I require 'parallel' but it's not installed. Please install it and try again."; kill -s 1 "$TOP_PID"; }
    command -v bc >/dev/null 2>&1 || { echo >&2 "I require 'bc' but it's not installed. Please install it and try again."; kill -s 1 "$TOP_PID"; }
    command -v timeout >/dev/null 2>&1 || { echo >&2 "I require 'timeout' but it's not installed. Please install it and try again."; kill -s 1 "$TOP_PID"; }
    osVersion="Linux"
  elif [[ "$(uname)" == "Darwin" ]]; then
    command -v jq >/dev/null 2>&1 || { echo >&2 "I require 'jq' but it's not installed. Please install it and try again."; kill -s 1 "$TOP_PID"; }
    command -v parallel >/dev/null 2>&1 || { echo >&2 "I require 'parallel' but it's not installed. Please install it and try again."; kill -s 1 "$TOP_PID"; }
    command -v bc >/dev/null 2>&1 || { echo >&2 "I require 'bc' but it's not installed. Please install it and try again."; kill -s 1 "$TOP_PID"; }
    command -v gtimeout >/dev/null 2>&1 || { echo >&2 "I require 'gtimeout' but it's not installed. Please install it and try again."; kill -s 1 "$TOP_PID"; }
    osVersion="Mac"
  fi
  echo "$osVersion"
}

fncValidateConfig() {
  local config="$1"
  if [[ -f "$config" ]]; then
    echo "reading config ..."
    configId=$(jq --raw-output .id "$config")
    configHost=$(jq --raw-output .host "$config")
    configPort=$(jq --raw-output .port "$config")
    configPath=$(jq --raw-output .path "$config")
    if ! [[ "$configId" ]] || ! [[ $configHost ]] || ! [[ $configPort ]] || ! [[ $configPath ]]; then
      echo "config is not correct"
      exit 1
    fi
  else
    echo "config file does not exist $config"
    exit 1
  fi
}

fncCreateDir() {
  local dirPath="${1}"
  [[ ! -d "$dirPath" ]] && mkdir -p "$dirPath"
}

fncProviderDefaults() {
  case "$provider" in
    cloudflare)
      [[ "$testHost" == "NULL" ]] && testHost="speed.cloudflare.com"
      [[ "$downloadPath" == "NULL" ]] && downloadPath="/__down?bytes="
      [[ "$uploadPath" == "NULL" ]] && uploadPath="/__up"
      ;;
    fastly|akamai)
      [[ "$testHost" == "NULL" ]] && testHost="$configHost"
      [[ "$downloadPath" == "NULL" ]] && downloadPath="/__down?bytes="
      [[ "$uploadPath" == "NULL" ]] && uploadPath="/__up"
      ;;
  esac
}

fncReadProviderSubnets() {
  local subnetsFile="$1" scriptDir="$2"
  local defaultSubnetsFileUrl defaultSubnetsFileUrlResult

  if [[ "$subnetsFile" != "NULL" ]]; then
    echo "Reading subnets from file $subnetsFile" >&2
    cat "$subnetsFile"
    return
  fi

  case "$provider" in
    cloudflare)
      defaultSubnetsFileUrl="https://raw.githubusercontent.com/MortezaBashsiz/CFScanner/main/config/cf.local.iplist"
      defaultSubnetsFileUrlResult=$(curl -I -L -s "$defaultSubnetsFileUrl" | grep "^HTTP" | grep 200 | awk '{ print $2 }')
      if [[ "$defaultSubnetsFileUrlResult" == "200" ]]; then
        echo "Reading subnets from $defaultSubnetsFileUrl" >&2
        curl -s "$defaultSubnetsFileUrl"
      else
        echo "URL $defaultSubnetsFileUrl is not available. Reading subnets from file $scriptDir/../config/cf.local.iplist" >&2
        cat "$scriptDir/../config/cf.local.iplist"
      fi
      ;;
    fastly)
      defaultSubnetsFileUrl="https://api.fastly.com/public-ip-list"
      echo "Reading Fastly subnets from $defaultSubnetsFileUrl" >&2
      curl -s "$defaultSubnetsFileUrl" | jq -r '.addresses[]'
      ;;
    akamai)
      echo "Akamai has no simple global public edge-IP list in this script. Use --file akamai.iplist" >&2
      exit 1
      ;;
  esac
}

fncMainCFFindSubnet() {
  local threads progressBar resultFile scriptDir configId configHost configPort configPath fileSize osVersion subnetsFile tryCount downThreshold upThreshold downloadOrUpload vpnOrNot quickOrNot
  threads="${1}"; progressBar="${2}"; resultFile="${3}"; scriptDir="${4}"; configId="${5}"; configHost="${6}"; configPort="${7}"; configPath="${8}"; fileSize="${9}"; osVersion="${10}"; subnetsFile="${11}"; tryCount="${12}"; downThreshold="${13}"; upThreshold="${14}"; downloadOrUpload="${15}"; vpnOrNot="${16}"; quickOrNot="${17}"
  local clientCommand parallelVersion cfSubnetList subNet breakedSubnets maxSubnet network netmask i breakedSubnet ipListLength passedIpsCount ipList

  if [[ "$osVersion" == "Linux" ]]; then
    [[ "$clientCore" == "XRAY" ]] && clientCommand="xray" || clientCommand="v2ray"
  elif [[ "$osVersion" == "Mac" ]]; then
    clientCommand="v2ray-mac"
  else
    echo "OS not supported only Linux or Mac"; exit 1
  fi

  parallelVersion=$(parallel --version | head -n1 | grep -Ewo '[0-9]{8}')
  cfSubnetList=$(fncReadProviderSubnets "$subnetsFile" "$scriptDir")

  ipListLength="0"
  for subNet in ${cfSubnetList}; do
    breakedSubnets=""; maxSubnet=24; network=${subNet%/*}; netmask=${subNet#*/}
    if [[ ${netmask} -ge ${maxSubnet} ]]; then
      breakedSubnets="${breakedSubnets} ${network}/${netmask}"
    else
      for i in $(seq 0 $(( (2 ** (maxSubnet - netmask)) - 1 ))); do
        breakedSubnets="${breakedSubnets} $(fncLongIntToStr $(( $(fncIpToLongInt "${network}") + (2 ** (32 - maxSubnet) * i) )))/${maxSubnet}"
      done
    fi
    breakedSubnets=$(echo "${breakedSubnets}" | tr ' ' '\n')
    for breakedSubnet in ${breakedSubnets}; do ipListLength=$((ipListLength+1)); done
  done

  passedIpsCount=0
  for subNet in ${cfSubnetList}; do
    breakedSubnets=""; maxSubnet=24; network=${subNet%/*}; netmask=${subNet#*/}
    if [[ ${netmask} -ge ${maxSubnet} ]]; then
      breakedSubnets="${breakedSubnets} ${network}/${netmask}"
    else
      for i in $(seq 0 $(( (2 ** (maxSubnet - netmask)) - 1 ))); do
        breakedSubnets="${breakedSubnets} $(fncLongIntToStr $(( $(fncIpToLongInt "${network}") + (2 ** (32 - maxSubnet) * i) )))/${maxSubnet}"
      done
    fi
    breakedSubnets=$(echo "${breakedSubnets}" | tr ' ' '\n')
    for breakedSubnet in ${breakedSubnets}; do
      fncShowProgress "$passedIpsCount" "$ipListLength"
      killall v2ray >/dev/null 2>&1
      killall xray >/dev/null 2>&1
      ipList=$(fncSubnetToIP "$breakedSubnet")
      tput cuu1; tput ed
      if [[ $parallelVersion -gt 20220515 ]]; then
        parallel --ll --bar -j "$threads" fncCheckIPList ::: "$ipList" ::: "$progressBar" ::: "$resultFile" ::: "$scriptDir" ::: "$configId" ::: "$configHost" ::: "$configPort" ::: "$configPath" ::: "$fileSize" ::: "$osVersion" ::: "$clientCommand" ::: "$tryCount" ::: "$downThreshold" ::: "$upThreshold" ::: "$downloadOrUpload" ::: "$vpnOrNot" ::: "$quickOrNot" ::: "$provider" ::: "$testHost" ::: "$downloadPath" ::: "$uploadPath"
      else
        echo -e "${RED}$progressBar${NC}"
        parallel -j "$threads" fncCheckIPList ::: "$ipList" ::: "$progressBar" ::: "$resultFile" ::: "$scriptDir" ::: "$configId" ::: "$configHost" ::: "$configPort" ::: "$configPath" ::: "$fileSize" ::: "$osVersion" ::: "$clientCommand" ::: "$tryCount" ::: "$downThreshold" ::: "$upThreshold" ::: "$downloadOrUpload" ::: "$vpnOrNot" ::: "$quickOrNot" ::: "$provider" ::: "$testHost" ::: "$downloadPath" ::: "$uploadPath"
      fi
      killall v2ray >/dev/null 2>&1
      killall xray >/dev/null 2>&1
      passedIpsCount=$((passedIpsCount+1))
    done
  done
  sort -n -k1 -t, "$resultFile" -o "$resultFile"
}

fncMainCFFindIP() {
  local threads progressBar resultFile scriptDir configId configHost configPort configPath fileSize osVersion IPFile tryCount downThreshold upThreshold downloadOrUpload vpnOrNot quickOrNot
  threads="${1}"; progressBar="${2}"; resultFile="${3}"; scriptDir="${4}"; configId="${5}"; configHost="${6}"; configPort="${7}"; configPath="${8}"; fileSize="${9}"; osVersion="${10}"; IPFile="${11}"; tryCount="${12}"; downThreshold="${13}"; upThreshold="${14}"; downloadOrUpload="${15}"; vpnOrNot="${16}"; quickOrNot="${17}"
  local clientCommand parallelVersion cfIPList

  if [[ "$osVersion" == "Linux" ]]; then
    [[ "$clientCore" == "XRAY" ]] && clientCommand="xray" || clientCommand="v2ray"
  elif [[ "$osVersion" == "Mac" ]]; then
    clientCommand="v2ray-mac"
  else
    echo "OS not supported only Linux or Mac"; exit 1
  fi

  parallelVersion=$(parallel --version | head -n1 | grep -Ewo '[0-9]{8}')
  cfIPList=$(cat "$IPFile")
  killall v2ray >/dev/null 2>&1
  killall xray >/dev/null 2>&1
  tput cuu1; tput ed
  if [[ $parallelVersion -gt 20220515 ]]; then
    parallel --ll --bar -j "$threads" fncCheckIPList ::: "$cfIPList" ::: "$progressBar" ::: "$resultFile" ::: "$scriptDir" ::: "$configId" ::: "$configHost" ::: "$configPort" ::: "$configPath" ::: "$fileSize" ::: "$osVersion" ::: "$clientCommand" ::: "$tryCount" ::: "$downThreshold" ::: "$upThreshold" ::: "$downloadOrUpload" ::: "$vpnOrNot" ::: "$quickOrNot" ::: "$provider" ::: "$testHost" ::: "$downloadPath" ::: "$uploadPath"
  else
    echo -e "${RED}$progressBar${NC}"
    parallel -j "$threads" fncCheckIPList ::: "$cfIPList" ::: "$progressBar" ::: "$resultFile" ::: "$scriptDir" ::: "$configId" ::: "$configHost" ::: "$configPort" ::: "$configPath" ::: "$fileSize" ::: "$osVersion" ::: "$clientCommand" ::: "$tryCount" ::: "$downThreshold" ::: "$upThreshold" ::: "$downloadOrUpload" ::: "$vpnOrNot" ::: "$quickOrNot" ::: "$provider" ::: "$testHost" ::: "$downloadPath" ::: "$uploadPath"
  fi
  killall v2ray >/dev/null 2>&1
  killall xray >/dev/null 2>&1
  sort -n -k1 -t, "$resultFile" -o "$resultFile"
}

clientConfigFile="https://raw.githubusercontent.com/MortezaBashsiz/CFScanner/main/config/ClientConfig.json"
subnetIPFile="NULL"

fncUsage() {
  echo -e "Usage: cfScanner [ -x|--core V2RAY/XRAY ] [ --provider cloudflare/fastly/akamai ] [ --test-host HOST ] [ --download-path PATH_PREFIX ] [ --upload-path PATH ] [ -v|--vpn-mode YES/NO ] [ -m|--mode SUBNET/IP ] [ -t|--test-type DOWN/UP/BOTH ] [ -p|--thread N ] [ -n|--tryCount N ] [ -c|--config FILE ] [ -s|--speed KB ] [ -r|--random N ] [ -d|--down-threshold N ] [ -u|--up-threshold N ] [ -f|--file FILE ] [ -q|--quick YES/NO ] [ -h|--help ]\n"
  echo -e "Defaults keep old Cloudflare behavior: --provider cloudflare --test-host speed.cloudflare.com --download-path /__down?bytes= --upload-path /__up"
  echo -e "Fastly example: ./cfScanner.sh --provider fastly --test-host speed.example.com --config ./config.json"
  echo -e "Akamai example: ./cfScanner.sh --provider akamai --test-host speed.example.com --file ./akamai.iplist --config ./config.json"
  exit 2
}

clientCore="XRAY"
provider="cloudflare"
testHost="NULL"
downloadPath="NULL"
uploadPath="NULL"
randomNumber="NULL"
downThreshold="1"
upThreshold="1"
osVersion="$(fncCheckDpnd)"
vpnOrNot="NO"
subnetOrIP="SUBNET"
downloadOrUpload="BOTH"
threads="4"
tryCount="1"
config="NULL"
speed="100"
quickOrNot="NO"

if [[ "$osVersion" == "Mac" ]]; then
  parsedArguments=$(getopt x:v:m:t:p:n:c:s:r:d:u:f:q:h "$@")
elif [[ "$osVersion" == "Linux" ]]; then
  parsedArguments=$(getopt -a -n cfScanner -o x:v:m:t:p:n:c:s:r:d:u:f:q:h --long core:,provider:,test-host:,download-path:,upload-path:,vpn-mode:,mode:,test-type:,thread:,tryCount:,config:,speed:,random:,down-threshold:,up-threshold:,file:,quick:,help -- "$@")
fi

eval set -- "$parsedArguments"

if [[ "$osVersion" == "Mac" ]]; then
  while :; do
    case "$1" in
      -x) clientCore="$2"; shift 2 ;;
      -v) vpnOrNot="$2"; shift 2 ;;
      -m) subnetOrIP="$2"; shift 2 ;;
      -t) downloadOrUpload="$2"; shift 2 ;;
      -p) threads="$2"; shift 2 ;;
      -n) tryCount="$2"; shift 2 ;;
      -c) config="$2"; shift 2 ;;
      -s) speed="$2"; shift 2 ;;
      -r) randomNumber="$2"; shift 2 ;;
      -d) downThreshold="$2"; shift 2 ;;
      -u) upThreshold="$2"; shift 2 ;;
      -f) subnetIPFile="$2"; shift 2 ;;
      -q) quickOrNot="$2"; shift 2 ;;
      -h) fncUsage ;;
      --) shift; break ;;
      *) echo "Unexpected option: $1 is not acceptable"; fncUsage ;;
    esac
  done
else
  while :; do
    case "$1" in
      -x|--core) clientCore="$2"; shift 2 ;;
      --provider) provider="$2"; shift 2 ;;
      --test-host) testHost="$2"; shift 2 ;;
      --download-path) downloadPath="$2"; shift 2 ;;
      --upload-path) uploadPath="$2"; shift 2 ;;
      -v|--vpn-mode) vpnOrNot="$2"; shift 2 ;;
      -m|--mode) subnetOrIP="$2"; shift 2 ;;
      -t|--test-type) downloadOrUpload="$2"; shift 2 ;;
      -p|--thread) threads="$2"; shift 2 ;;
      -n|--tryCount) tryCount="$2"; shift 2 ;;
      -c|--config) config="$2"; shift 2 ;;
      -s|--speed) speed="$2"; shift 2 ;;
      -r|--random) randomNumber="$2"; shift 2 ;;
      -d|--down-threshold) downThreshold="$2"; shift 2 ;;
      -u|--up-threshold) upThreshold="$2"; shift 2 ;;
      -f|--file) subnetIPFile="$2"; shift 2 ;;
      -q|--quick) quickOrNot="$2"; shift 2 ;;
      -h|--help) fncUsage ;;
      --) shift; break ;;
      *) echo "Unexpected option: $1 is not acceptable"; fncUsage ;;
    esac
  done
fi

validArguments=$?
if [[ "$validArguments" != "0" ]]; then echo "error validate"; exit 2; fi

provider=$(echo "$provider" | tr '[:upper:]' '[:lower:]')
clientCore=$(echo "$clientCore" | tr '[:lower:]' '[:upper:]')
vpnOrNot=$(echo "$vpnOrNot" | tr '[:lower:]' '[:upper:]')
subnetOrIP=$(echo "$subnetOrIP" | tr '[:lower:]' '[:upper:]')
downloadOrUpload=$(echo "$downloadOrUpload" | tr '[:lower:]' '[:upper:]')
quickOrNot=$(echo "$quickOrNot" | tr '[:lower:]' '[:upper:]')

[[ "$clientCore" != "XRAY" && "$clientCore" != "V2RAY" ]] && { echo "Wrong value: $clientCore Must be XRAY or V2RAY"; exit 2; }
[[ "$provider" != "cloudflare" && "$provider" != "fastly" && "$provider" != "akamai" ]] && { echo "Wrong value: $provider Must be cloudflare or fastly or akamai"; exit 2; }
[[ "$vpnOrNot" != "YES" && "$vpnOrNot" != "NO" ]] && { echo "Wrong value: $vpnOrNot Must be YES or NO"; exit 2; }
[[ "$subnetOrIP" != "SUBNET" && "$subnetOrIP" != "IP" ]] && { echo "Wrong value: $subnetOrIP Must be SUBNET or IP"; exit 2; }
[[ "$downloadOrUpload" != "DOWN" && "$downloadOrUpload" != "UP" && "$downloadOrUpload" != "BOTH" ]] && { echo "Wrong value: $downloadOrUpload Must be DOWN or UP or BOTH"; exit 2; }
[[ "$quickOrNot" != "YES" && "$quickOrNot" != "NO" ]] && { echo "Wrong value: $quickOrNot Must be YES or NO"; exit 2; }

if [[ "$subnetIPFile" != "NULL" && ! -f "$subnetIPFile" ]]; then
  echo "file does not exists: $subnetIPFile"
  exit 1
fi

now=$(date +"%Y%m%d-%H%M%S")
scriptDir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &>/dev/null && pwd)
resultDir="$scriptDir/result"
resultFile="$resultDir/$now-result.cf"
tempConfigDir="$scriptDir/tempConfig"
filesDir="$tempConfigDir"
uploadFile="$filesDir/upload_file"
configId="NULL"
configHost="NULL"
configPort="NULL"
configPath="NULL"
progressBar=""

export GREEN='\033[0;32m'
export BLUE='\033[0;34m'
export RED='\033[0;31m'
export ORANGE='\033[0;33m'
export YELLOW='\033[1;33m'
export NC='\033[0m'
export provider testHost downloadPath uploadPath clientCore randomNumber osVersion progressBar GREEN BLUE RED ORANGE YELLOW NC

fncCreateDir "${resultDir}"
fncCreateDir "${tempConfigDir}"
echo "" > "$resultFile"

if [[ "$config" == "NULL" ]]; then
  echo "updating config"
  configRealUrlResult=$(curl -I -L -s "$clientConfigFile" | grep "^HTTP" | grep 200 | awk '{ print $2 }')
  if [[ "$configRealUrlResult" == "200" ]]; then
    curl -s "$clientConfigFile" -o "$scriptDir/config.default"
    echo "config.default updated with $clientConfigFile"
    echo ""
    config="$scriptDir/config.default"
    cat "$config"
  else
    echo ""
    echo "config file is not available $clientConfigFile"
    echo "use your own"
    echo ""
    exit 1
  fi
else
  echo ""
  echo "using your own config $config"
  cat "$config"
  echo ""
fi

fncValidateConfig "$config"
fncProviderDefaults

fileSize="$((2*speed*1024))"

if [[ "$provider" != "cloudflare" ]]; then
  echo "Provider: $provider"
  echo "Test host: $testHost"
  echo "Download URL format: https://$testHost${downloadPath}<bytes>"
  echo "Upload URL: https://$testHost${uploadPath}"
fi

if [[ "$downloadOrUpload" == "DOWN" || "$downloadOrUpload" == "BOTH" ]]; then
  echo "You are testing download"
fi
if [[ "$downloadOrUpload" == "UP" || "$downloadOrUpload" == "BOTH" ]]; then
  echo "You are testing upload"
  echo "making upload file by size $fileSize Bytes in $uploadFile"
  ddSize="$((2*speed))"
  dd if=/dev/random of="$uploadFile" bs=1024 count="$ddSize" >/dev/null 2>&1
fi

if [[ "$subnetOrIP" == "SUBNET" ]]; then
  fncMainCFFindSubnet "$threads" "$progressBar" "$resultFile" "$scriptDir" "$configId" "$configHost" "$configPort" "$configPath" "$fileSize" "$osVersion" "$subnetIPFile" "$tryCount" "$downThreshold" "$upThreshold" "$downloadOrUpload" "$vpnOrNot" "$quickOrNot"
elif [[ "$subnetOrIP" == "IP" ]]; then
  if [[ "$subnetIPFile" == "NULL" ]]; then
    echo "IP mode needs -f|--file"
    exit 1
  fi
  fncMainCFFindIP "$threads" "$progressBar" "$resultFile" "$scriptDir" "$configId" "$configHost" "$configPort" "$configPath" "$fileSize" "$osVersion" "$subnetIPFile" "$tryCount" "$downThreshold" "$upThreshold" "$downloadOrUpload" "$vpnOrNot" "$quickOrNot"
else
  echo "$subnetOrIP is not correct choose one SUBNET or IP"
  exit 1
fi
