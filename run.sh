#!/bin/bash

if [ `whoami` != root ]; then
    echo "Please run this script as root or using sudo"
    exit
fi

if [ "$#" -ne 1 ]; then
    echo "Please add USER argument(sudo sh run.sh $ + USER)"
    exit
fi

echo $1;
echo "Run programm and server..Please..wait"

su $1 xdg-open dist/index.html ; sudo ./vhost-manager ; 