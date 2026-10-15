# BMS - Battery Management System

## Functions

- Decide if the battery needs charging or PV of the day is enough to load the battery.
- Find the lowest price of electricity from the grid and charge the battery if needed.
- If too much PV energy is available, discharge the battery pre MAX PV.

- A simulation mode will display the changes but does not control the battery. It is used to test the BMS logic and see how it would behave in different scenarios.
- Inputs are ready via sensors or inputs values from Home Assistant.
- Outputs are sent to Home Assistant to control the battery and other devices.
- Max AC output will be monitored, if not intended by the BMS, the BMS will reduce it
- Berechnung der zeiträume mit geringen kosten


## ToDo of the Day
[x] Add input power
[x] Add control mode auto
[x] Add auto modes
[x] Change state update -> central function
[x] Get expected PV energy of the day
[x] Calc missing battery energy
[x] Calc loading amount (best spot)
[x] Start and stop loading the battery
[x] Input for max battery energy
[x] subscription chan with initial value
