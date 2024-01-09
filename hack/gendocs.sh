#!/usr/bin/env bash
# Copyright 2024 Chmouel Boudjnah <chmouel@chmouel.com>
set -eufo pipefail
dir=/var/www/docs

mkdir -p ${dir}
if [[ -d ${dir}/git ]]; then
	cd ${dir}/git
	git reset --hard
	git pull --all
	git clean -f .
else
	git clone --tags https:///github.com/openshift-pipelines/pipelines-as-code.git ${dir}/git
	cd ${dir}/git
fi

versiondir=${dir}/versions
mkdir -p ${versiondir}

declare -A hashmap=()
for i in $(git tag -l | grep '^v' | sort -V); do
	version=${i//v/}
	if [[ ${version} =~ ^([0-9]+\.[0-9]+)\.[0-9]+$ ]]; then
		major_version=${BASH_REMATCH[1]}
	fi
	hashmap["$major_version"]=$version
done
output=$(for i in "${!hashmap[@]}"; do
	echo v"${hashmap[$i]}"
done | sort -rV | tr "\n" " ")
allversiontags=${output// /,}

for i in $output; do
	version=${versiondir}/${i}
	[[ -d ${version} ]] && continue
	git checkout -B gendoc origin/release-$i
	echo ${allversiontags} >docs/content/ALLVERSIONS
	find docs/content -name '*.md' -print0 | xargs -0 sed -i 's,/images/,../../../images/,g'
	sed -i 's,BookLogo = "/,BookLogo = ",' docs/config.toml
	sed -i 's,window.location =.*https://release-.*.pipelines-as-code.pages.dev".*,window.location = "https://docs.pipelinesascode.com/" + elm.value + "/" + current_page_path.split("/").slice(2).join("/"),' docs/layouts/partials/docs/footer.html
	mkdir -p ${version}
	hugo -d ${version} -s docs -b https://docs.pipelinesascode.com/$i
	git reset --hard
	git clean -f .
done

# generate simple HTML
cat <<EOF >${versiondir}/index.html
<html>
<head>
<title>OpenShift Pipelines as Code versions Documentation</title>
</head>
<body>
<div style="text-align: center;width: 100%">
<h1>Pipeline as Code versions Documentation</h1>
EOF

for i in $output; do
	cat <<EOF >>${versiondir}/index.html
  <a href="./${i}">${i}</a><br>
EOF
done

cat <<EOF >>${versiondir}/index.html
</div></body></html>
EOF
