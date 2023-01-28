#! /usr/bin/python

import sys
import os
import getopt
import subprocess
import plistlib
import operator

class CheckException (Exception):
    """
    Raised when the "check" subcommand detects a problem; the top-level code catches 
    this and prints a nice error message.
    """
    def __init__(self, message, path=None):
        self.message = message
        self.path = path

def readPlistFromToolSection(toolPath, segmentName, sectionName):
    """Reads a dictionary property list from the specified section within the specified executable."""
    
    # Run otool -s to get a hex dump of the section.
    
    args = [
        # "false", 
        "otool", 
        "-s", 
        segmentName, 
        sectionName, 
        toolPath
    ]
    try:
        plistDump = subprocess.check_output(args)
    except (subprocess.CalledProcessError, e):
        raise CheckException("tool %s / %s section unreadable" % (segmentName, sectionName), toolPath)

    # Convert that hex dump to an property list.
    
    plistLines = plistDump.splitlines()
    if len(plistLines) < 3 or plistLines[1] != ("Contents of (%s,%s) section" % (segmentName, sectionName)):
        raise CheckException("tool %s / %s section dump malformed (1)" % (segmentName, sectionName), toolPath)
    del plistLines[0:2]

    try:
        bytes = []
        for line in plistLines:
            # line looks like this:
            #
            # '0000000100000b80\t3c 3f 78 6d 6c 20 76 65 72 73 69 6f 6e 3d 22 31 '
            columns = line.split("\t")
            assert len(columns) == 2
            for hexStr in columns[1].split():
                bytes.append(int(hexStr, 16))
        plist = plistlib.readPlistFromString(bytearray(bytes))
    except:
        raise CheckException("tool %s / %s section dump malformed (2)" % (segmentName, sectionName), toolPath)

    # Check the root of the property list.
    
    if not isinstance(plist, dict):
        raise CheckException("tool %s / %s property list root must be a dictionary" % (segmentName, sectionName), toolPath)

    return plist


plist = readPlistFromToolSection(sys.argv[1], "__TEXT", "__info_plist")
print(plist)