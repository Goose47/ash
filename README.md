# ASH

## Description
ASH (*A*lias s*SH*) is a tool that remembers ssh configuration and alias.

## Usage
Usage is similar to ssh command

``ash -p 22 user@host.com alias``

After this host may be accessed via alias

``ash alias``

Supported flags
- -i
- -p

## Requirements
go 1.23.4+

## Installation

``go install .\cmd\ash\ash.go``

[//]: # (todo: password encryption)
[//]: # (todo: format error messages)
[//]: # (todo: tests)