wait (){
  counter=0
  while [ "$(pgrep capidaemon)" != "" ]; do
    counter=$((counter+1))
    if [[ "$counter" -gt 60 ]]; then
      break
    fi
    sleep 1
  done
}

pkill -2 capidaemon
wait

if [ "$(pgrep capidaemon)" == "" ]; then
  echo pkill -2 succeeded
else
  echo pkill -2 failed
  pkill -9 capidaemon 2>&1
  wait
  if [ "$(pgrep capidaemon)" == "" ]; then
    echo pkill -9 succeeded
  else
    echo pkill -9 failed
    exit 9
  fi
fi

