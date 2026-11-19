- [Disclaimer](#disclaimer)
- [General information](#general-information)
- [Some found commands](#some-found-commands)
- [Some queries and answers](#some-queries-and-answers)
  - [Set Interval](#set-interval)
  - [Set Settings](#set-settings)
  - [Set Date](#set-date)
  - [Set Callibration](#set-callibration)
  - [Get Sensors](#get-sensors)

# Disclaimer
Translated by Google, sorry :)


# General information
Report Id(0x00) is added to the code; it is mentioned here and will not be taken into account.

Host=\>device / device=\>host exchange packets begin with byte 0x7b.

In host=\>device packets, the second byte is the command (1 byte). The data block ends with two bytes - 0x40 + 0x7d.

There is no command in device=\>host packets, and the data starts from the 2nd byte. The data block ends with two bytes - 0x40 + 0x7d. In some packets from the device, 0x407b occurs several times, perhaps we are talking about several blocks of data at a time.

# Some found commands
- 0x04 host=\>device, Get Settings
- 0x05 host=\>device, Get Calibrations
- 0x10 host=\>device, Set Settings
- 0x11 host=\>device, Set Callibration
- 0x30 host=\>device, Set Date
- 0x40 host=\>device, Set Interval
- 0x03 host=\>device, Get Sensors

For the exporter, only 0x03, 0x30 were used.

# Some queries and answers

## Set Interval

host=\>device: **7b4001407d**

byte(3) - interval for recording sensor data in a CSV file on a memory card, in minutes

device=\>host: 7b2f407d

What does 0x2f mean in the answer is not clear, because does not depend on the sent value.

## Set Settings

Display settings are the same as in EasyTemp in the Settings window, except Interval (see above).

host=\>device: 7b10000c00000003000000000100000008000000400000020000001000407d

byte(2) = 0x10 - command flag

byte(3) - Graph Type Temperature
- **0x00 - Temperature**
- 0x01 - Humidity
- 0x02 - Dew Point
- 0x03 - Heat Index

byte(4) - Graph Time(in general, the period of graphics display in hours, converted to hex):
- **0x0c - 12H**
- 0x18 - 24H
- 0x30 - 48H
- 0x42 - 72H

byte(5) - Time Format
- 0x00 - h:mm:ss
- 0x01 - AM h:mm:ss
- 0x02 - h:mm:ss AM

- byte(5) - Date Format
- 0x00 - YYYY-MM-DD
- 0x01 - MM-DD-YYYY
- 0x02 - DD-MM-YYYY

byte(7) - DST(this setting is not saved in my device)
- **0x00 - OFF**
- 0x01 - ON

byte(8) - TimeZone, INT
- dec =\> hex 0...12H
- 256-dec =\> hex -1...-12H

byte(9) - Units
- 0x00 - C
- 0x01 - F

byte(13) - CH1
- 0x01 - Temperature
- 0x02 - DewPoint
- 0x03 - Temperature + DewPoint
- 0x04 - HeatIndex
- 0x05 - Temperature + HeatIndex
- 0x06 - DewPoint + HeatIndex
- 0x07 - Temperature + DewPoint + HeatIndex

byte(14)...byte(20) - CH2...CH8

byte(21)...byte(29) - didn't understand

byte(30)+byte(31) - EoD flag(0x407d)


## Set Date
Setting the clock on the base

host=\>device: 7b3007ea0a1b0e0b0a02407d

byte(1)

byte(2) =\> 0x30, command flag

byte(3) & byte(4) =\> комбинация значений из byte(3) и byte(4) соответствует году, т.е. 2026 год это byte(2)=0x07 и byte(3) = 0xEA

byte(5) =\> month dec=\>hex

byte(6) =\> day dec=\>hex

byte(7) =\> hours(24H) dec=\>hex

byte(8) =\> minutes dec=\>hex

byte(9) =\> secunds dec=\>hex

byte(10) = 0x02 =\> назначение в процессе сниффинга не опознано

byte(11) & byte(12) =\> флаг EoD(0x407d)

## Set Callibration

Storing  data for sensors calibration. It looks like it is being written to ROM, so it’s better not to write it to often.

host=\>device: 7b11ffecfb000000000000000000000000000000000000000000407d

byte(1)

byte(2) =\> 0x11, command flag

Then there are 8 groups (according to the number of channels, in the example of correction for only one sensor, for testing) of three bytes each. The first two bytes of the group are temperature, the third byte is humidity.

Temperature(t):

For temperatures from 0 and above - a simple conversion of t\*10 to hex.

For a negative correction, we get the value like this: t\*10+65536, and convert it to hex. Example for amendment -2C:\
`-2\*10+65536 =\> 65516 =\> 0xffec =\> first byte of group 0xff, second 0xec`

Humidity (h) is similar (taking into account the size of 1 byte):

For correction from 0 and above (in percentage) - a simple translation into hex.

For a negative correction h - (h+256)=\>hex

For correction -5%:\
`-5+256 = 251 =\> 0xfb`

## Get Sensors

host=\>device: 7b03407d

device=\>host: **7b010b117fffff7fffff7fffff7fffff7fffff7fffff7fffff407d***0064012c0064407d000a7d696e742843292c48656174496e6465782843290d0a7d3e48b2d5*

byte(1) - as usual(0x7b)

Then there are 8 groups (according to the number of channels) of three bytes each.

First, two bytes temperature (see Set Callibration for how to interpret), then one byte humidity (here just dec =\> hex).

If the sensor is not connected, it will be 0x7fffff.

I didn’t understand what kind of data wireshark caught after the first turn on 0x407d.