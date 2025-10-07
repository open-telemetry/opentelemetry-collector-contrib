# Test Configuration Files

This directory contains example configuration files for testing the Cisco OS receiver.

---

## 📄 Configuration Files

### **1. config-single-device.yaml**
**Minimal configuration for monitoring a single Cisco device**

- ✅ Simplest possible configuration
- ✅ One device, default metrics
- ✅ Good for initial testing

**Usage:**
```bash
./otelcol-custom --config receiver/ciscoosreceiver/testdata/config-single-device.yaml
```

---

### **2. config-multi-device.yaml** ⭐ **RECOMMENDED**
**Correct pattern for monitoring multiple devices**

- ✅ Separate receiver instance per device
- ✅ Devices monitored in parallel
- ✅ Production-ready pattern
- ✅ Each device gets its own SSH connection pool

**Key Features:**
- `ciscoosreceiver/device1` - Named instance for device 1
- `ciscoosreceiver/device2` - Named instance for device 2
- Both devices in same pipeline
- Resource attributes added via processor

**Usage:**
```bash
./otelcol-custom --config receiver/ciscoosreceiver/testdata/config-multi-device.yaml
```

---

### **3. config-single-instance-multi-device.yaml** ⚠️ **ANTI-PATTERN**
**Demonstrates incorrect configuration (for testing warning)**

- ❌ Multiple devices in ONE receiver instance
- ⚠️ Only first device will be monitored
- ⚠️ Triggers warning log message
- 🧪 Use for testing warning message only

**Expected Warning:**
```
WARN Multiple devices configured in single receiver instance - only first device will be monitored
     device_count=2
     monitoring_device=10.106.0.5
     recommendation="Use separate receiver instances for multiple devices"
```

**Usage (for testing):**
```bash
./otelcol-custom --config receiver/ciscoosreceiver/testdata/config-single-instance-multi-device.yaml
# You should see the warning in logs
```

---

## 🎯 Quick Comparison

| Configuration | Devices | Pattern | Use Case |
|--------------|---------|---------|----------|
| `config-single-device.yaml` | 1 | ✅ Correct | Simple testing |
| `config-multi-device.yaml` | 2+ | ✅ Correct | Production |
| `config-single-instance-multi-device.yaml` | 2+ in 1 | ❌ Wrong | Testing warnings |

---

## 🚀 Testing Commands

### **Test Single Device:**
```bash
cd /Users/erden/codebase/forks/Untitled
./dist/otelcol-custom --config receiver/ciscoosreceiver/testdata/config-single-device.yaml
```

### **Test Multiple Devices (Correct Pattern):**
```bash
./dist/otelcol-custom --config receiver/ciscoosreceiver/testdata/config-multi-device.yaml
```

### **Test Warning Message:**
```bash
./dist/otelcol-custom --config receiver/ciscoosreceiver/testdata/config-single-instance-multi-device.yaml
# Look for WARN log about multiple devices
```

---

## 📝 Configuration Notes

### **Authentication:**
All example configs use:
- Username: `admin`
- Password: `password`

**⚠️ Update these for your actual devices!**

### **Device IPs:**
- Device 1: `192.168.1.1`
- Device 2: `192.168.1.2`

**⚠️ Update these for your actual network!**

### **Collection Interval:**
- Default: `30s`
- Adjust based on your needs (10s-5m typical range)

---

## ✅ Best Practices

1. **One device per receiver instance** - Use `/device1`, `/device2` naming
2. **Use resource processor** - Add environment, cluster tags
3. **Enable batch processor** - Improves efficiency
4. **Set appropriate timeouts** - SSH + command execution time
5. **Monitor both scrapers** - system (connectivity) + interfaces (metrics)

---

## 🐛 Troubleshooting

### **Connection Refused:**
- Check device IP and port
- Verify SSH is enabled on device
- Check network connectivity

### **Authentication Failed:**
- Verify username/password
- Check device AAA configuration
- Try SSH key authentication

### **Only First Device Monitored:**
- Check if you used single instance with multiple devices
- Look for WARN message in logs
- Use separate receiver instances instead

---

## 📚 Additional Resources

- [Receiver Documentation](../README.md)
- [OpenTelemetry Collector Config](https://opentelemetry.io/docs/collector/configuration/)
