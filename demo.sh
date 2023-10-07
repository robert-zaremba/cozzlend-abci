#!/usr/bin/env sh


APP=/home/robert/go/src/github.com/cosmos/cosmos-sdk/build/simd
CHAIN_DIR="/home/robert/.simapp"
chainid="cosmos1"
n0cfgDir=$CHAIN_DIR/config
n0cfg="$n0cfgDir/config.toml"
n0app="$n0cfgDir/app.toml"

CLEANUP="${CLEANUP:-1}"
if [[ "$CLEANUP" == 1 || "$CLEANUP" == "1" ]]; then
  rm -rf "$CHAIN_DIR"
  echo "Removed $CHAIN_DIR"
fi

VAL0_KEY="val"
VAL0_MNEMONIC="sing exist switch change blind exhaust clog net diet emotion host stem opinion sort eagle unit flight east pepper drastic crowd cram snap require"
VAL0_ADDR="cosmos1tv3g90fr96284f9nrx6ps97l8fdgm5kzp8l5s7"

USER1_KEY="user1"
USER1_MNEMONIC="medal depth balcony equal abuse glass issue perfect kitchen genre finish alert polar genius february fiber leader connect tuna army caught spring pretty lucky"
USER1_ADDR="cosmos1rzx048gyn0z4572m2qjxg2ufv53z7wq4mmnnv6"

USER2_KEY="user2"
USER2_MNEMONIC="taxi comfort awake punch fuel copper alpha error math next plunge arrest quarter rent source nominee when arrive glide uniform item father receive elephant"
USER2_ADDR="cosmos1f2xzyj84l3jlve24eysjrehuswyjp3r7m3tf2w"

USER3_KEY="user3"
USER3_MNEMONIC="depend utility document trigger check announce joy prepare often bonus either half report credit mad husband craft pass wall equip mandate divorce sheriff dutch"
USER3_ADDR="cosmos1tdj93x9r4n9kl6az93r05l2v9fxphkn5guqyhw"

scale_factor="000000"
val_coins="3000${scale_factor}stake"
val_coins_del="30${scale_factor}stake"
coins="1000000${scale_factor}uatom,10000${scale_factor}stake"


$APP init mon1 --chain-id $chainid &>/dev/null


echo "--- Patching config..."

perl -i -pe 's|timeout_commit = ".*?"|timeout_commit = "2s"|g' "$n0cfg"
# enable API
sed -i -s '119s/enable = false/enable = true/' "$n0app"



echo "--- Importing keys..."
NEWLINE=$'\n'

# $APP keys add $VAL0_KEY  --recover


yes "$VAL0_MNEMONIC$NEWLINE"  | $APP keys add $VAL0_KEY  --recover
yes "$USER1_MNEMONIC$NEWLINE" | $APP keys add $USER1_KEY --recover
yes "$USER2_MNEMONIC$NEWLINE" | $APP keys add $USER2_KEY --recover
yes "$USER3_MNEMONIC$NEWLINE" | $APP keys add $USER3_KEY --recover

echo "--- creating gen accounts..."

$APP genesis add-genesis-account $VAL0_ADDR $val_coins
$APP genesis add-genesis-account $USER1_ADDR $coins &>/dev/null
$APP genesis add-genesis-account $USER2_ADDR $coins &>/dev/null
$APP genesis add-genesis-account $USER3_ADDR $coins &>/dev/null


$APP genesis gentx val $val_coins_del --chain-id $chainid
$APP genesis collect-gentxs
$APP start

exit


export SIMD_CHAIN_ID=$chainid



#################################
##    DEMO
#################################

simd tx bank send -y user2 cosmos1f2xzyj84l3jlve24eysjrehuswyjp3r7m3tf2w 20uatom & \
simd tx bank liquidate -y user1 cosmos1f2xzyj84l3jlve24eysjrehuswyjp3r7m3tf2w 20stake


simd q bank balances $(simd keys show user1 -a)

simd tx bank liquidate -y user1 cosmos1f2xzyj84l3jlve24eysjrehuswyjp3r7m3tf2w 200stake & \
simd tx bank liquidate -y user2 cosmos1f2xzyj84l3jlve24eysjrehuswyjp3r7m3tf2w 300stake


simd q bank balances $(simd keys show user1 -a)
