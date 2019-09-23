import { useEffect, useReducer, useState }  from 'react';

const SERVICE_ID = 'ca000000-75dd-4a0e-b688-66b7df342cc6';
const DEVICE_NAME = 0x2A00;
const MANUFACTURER_NAME = 0x2A29;
// const SERIAL_NUMBER = 0x2A25;
const MODEL_NUMBER = 0x2A24;
const NETWORK_AVAILABILITY_STATUS = 'ca000001-75dd-4a0e-b688-66b7df342cc6';
const IP_ADDRESS = 'ca000002-75dd-4a0e-b688-66b7df342cc6';
const WIFI_SCAN_SIGNAL = 'ca000003-75dd-4a0e-b688-66b7df342cc6';
const DISCOVERED_WIFI_SSID = 'ca000004-75dd-4a0e-b688-66b7df342cc6';
const WIFI_SSID_STRING = 'ca000005-75dd-4a0e-b688-66b7df342cc6';
const WIFI_PSK_STRING = 'ca000006-75dd-4a0e-b688-66b7df342cc6';
const WIFI_CONNECT_SIGNAL = 'ca000007-75dd-4a0e-b688-66b7df342cc6';
const ONION_API = 'ca000008-75dd-4a0e-b688-66b7df342cc6';

function supported() {
  return 'bluetooth' in navigator;
}

async function characteristicByte(characteristic) {
  const value = await characteristic.readValue();
  return byte(value);
}

async function characteristicText(characteristic) {
  const value = await characteristic.readValue();
  return text(value);
}

async function byte(value) {
  return value.getUint8(0);
}

async function text(value) {
  return new TextDecoder('utf-8').decode(value);
}

function dispenserReducer(state = null, patch) {
  if (!patch) {
    return null;
  }

  return {
    ...(state || {}),
    ...patch,
  };
}

// TODO(davidknezic) remove this
function availableWifisReducer(state = [], action) {
  console.log(action);

  if (action) {
    state = action.split('\t');
  }

  return state;
}

export function useBluetooth() {
  const [dispenser, setDispenser] = useReducer(dispenserReducer);
  const [connecting, setConnecting] = useState(false);
  const [server, setServer] = useState(false);
  const [force, forceUpdate] = useReducer(x => x + 1, 0);
  const [availableWifis, dispatchAvailableWifisAction] = useReducer(availableWifisReducer);

  async function connect() {
    setConnecting(true);

    const device = await navigator.bluetooth.requestDevice({
      filters: [{
        services: [SERVICE_ID],
      }],
    });

    const server = await device.gatt.connect();

    console.log(server);

    device.ongattserverdisconnected = disconnected;

    setServer(server);

    setConnecting(false);
  }

  function disconnected(event) {
    const device = event.target;

    console.log('disconnected', device);

    setServer(null);
  }

  async function disconnect() {
    await server.disconnect();
  }

  async function scanWifi() {
    const service = await server.getPrimaryService(SERVICE_ID);
    const wifiScanSignalCharacteristic = await service.getCharacteristic(WIFI_SCAN_SIGNAL);

    await wifiScanSignalCharacteristic.writeValue(Uint8Array.of(1));
  }

  async function connectWifi(ssid, psk) {
    try {
      const service = await server.getPrimaryService(SERVICE_ID);

      const [
        wifiSsidStringCharacteristic,
        wifiPskStringCharacteristic,
        wifiConnectSignalCharacteristic,
      ] = await Promise.all([
        await service.getCharacteristic(WIFI_SSID_STRING),
        await service.getCharacteristic(WIFI_PSK_STRING),
        await service.getCharacteristic(WIFI_CONNECT_SIGNAL),
      ]);

      // const wifiSsidStringCharacteristic = await service.getCharacteristic(WIFI_SSID_STRING);
      // const wifiPskStringCharacteristic = await service.getCharacteristic(WIFI_PSK_STRING);
      // const wifiConnectSignalCharacteristic = await service.getCharacteristic(WIFI_CONNECT_SIGNAL);

      await wifiSsidStringCharacteristic.writeValue(new TextEncoder('utf-8').encode(ssid));
      await wifiPskStringCharacteristic.writeValue(new TextEncoder('utf-8').encode(psk));
      await wifiConnectSignalCharacteristic.writeValue(Uint8Array.of(1));
    } catch (e) {
      console.log(`unable to connect wifi: ${e}`);
    }
  }

  useEffect(() => {
    console.log('updating...');

    const subscriptions = [];

    async function update() {
      if (!server) {
        setDispenser(null);
        return;
      }

      try {
        const service = await server.getPrimaryService(SERVICE_ID);

        const [
          deviceNameCharacteristic,
          manufacturerNameCharacteristic,
          // serialNumberCharacteristic,
          modelNumberCharacteristic,
          networkAvailabilityStatusCharacteristic,
          ipAddressCharacteristic,
          discoveredWifiSsidCharacteristic,
          wifiSsidStringCharacteristic,
          onionApiCharacteristic,
        ] = await Promise.all([
          await service.getCharacteristic(DEVICE_NAME),
          await service.getCharacteristic(MANUFACTURER_NAME),
          // await service.getCharacteristic(SERIAL_NUMBER),
          await service.getCharacteristic(MODEL_NUMBER),
          await service.getCharacteristic(NETWORK_AVAILABILITY_STATUS),
          await service.getCharacteristic(IP_ADDRESS),
          await service.getCharacteristic(DISCOVERED_WIFI_SSID),
          await service.getCharacteristic(WIFI_SSID_STRING),
          await service.getCharacteristic(ONION_API),
        ]);

        // const deviceNameCharacteristic = await service.getCharacteristic(DEVICE_NAME);
        // const manufacturerNameCharacteristic = await service.getCharacteristic(MANUFACTURER_NAME);
        // // const serialNumberCharacteristic = await service.getCharacteristic(SERIAL_NUMBER);
        // const modelNumberCharacteristic = await service.getCharacteristic(MODEL_NUMBER);
        // const networkAvailabilityStatusCharacteristic = await service.getCharacteristic(NETWORK_AVAILABILITY_STATUS);
        // const ipAddressCharacteristic = await service.getCharacteristic(IP_ADDRESS);
        // const wifiScanListCharacteristic = await service.getCharacteristic(WIFI_SCAN_LIST);
        // const wifiSsidStringCharacteristic = await service.getCharacteristic(WIFI_SSID_STRING);

        const [
          deviceName,
          manufacturerName,
          // serialNumber,
          modelNumber,
          networkAvailabilityStatus,
          ipAddress,
          ssidString,
          onionApi,
        ] = await Promise.all([
          characteristicText(deviceNameCharacteristic),
          characteristicText(manufacturerNameCharacteristic),
          // characteristicText(serialNumberCharacteristic),
          characteristicText(modelNumberCharacteristic),
          characteristicByte(networkAvailabilityStatusCharacteristic),
          characteristicText(ipAddressCharacteristic),
          characteristicText(wifiSsidStringCharacteristic),
          characteristicText(onionApiCharacteristic),
        ]);

        await networkAvailabilityStatusCharacteristic.startNotifications();
        networkAvailabilityStatusCharacteristic.oncharacteristicvaluechanged = async (event) => {
          setDispenser({
            networkAvailabilityStatus: byte(event.target.value),
          });
        };
        subscriptions.push(networkAvailabilityStatusCharacteristic);

        await wifiSsidStringCharacteristic.startNotifications();
        wifiSsidStringCharacteristic.oncharacteristicvaluechanged = async (event) => {
          setDispenser({
            ssidString: text(event.target.value),
          });
        };
        subscriptions.push(wifiSsidStringCharacteristic);

        await discoveredWifiSsidCharacteristic.startNotifications();
        discoveredWifiSsidCharacteristic.oncharacteristicvaluechanged = async (event) => {
          dispatchAvailableWifisAction(text(event.target.value));
        };
        subscriptions.push(discoveredWifiSsidCharacteristic);

        setDispenser({
          deviceName,
          manufacturerName,
          // serialNumber,
          modelNumber,
          networkAvailabilityStatus,
          ipAddress,
          ssidString,
          onionApi,
        });
      } catch (e) {
        console.error(`unable to read: ${e}`);
      }
    }
    update();

    return function cleanup() {
      console.log('cleanup...');
      Promise.all(subscriptions.map(subscription => async () => {
        subscription.oncharacteristicvaluechanged = null;
        await subscription.stopNotifications();
      }));
    };
  }, [server, force]);

  return {
    supported,
    connecting,
    dispenser,
    availableWifis,
    connect,
    disconnect,
    forceUpdate,
    scanWifi,
    connectWifi,
  };
}
