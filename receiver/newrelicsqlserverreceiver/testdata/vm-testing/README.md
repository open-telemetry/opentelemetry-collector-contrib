# ✅ **WORKING SOLUTION - Ready to Use!**

Your VM testing setup is ready! Here's what you have:

## 📁 **Files** 
```
testdata/vm-testing/
├── config.yaml        # OpenTelemetry config with newrelicsqlserverreceiver + New Relic
├── run.sh             # Runs the collector with your receiver (WORKING!)
└── README.md          # This file
```

## 🚀 **How to Use**

### 1. Set Your Credentials
```bash
export SQLSERVER_HOST="your-sql-server-ip"           # e.g., "192.168.1.100"  
export SQLSERVER_USERNAME="your-sql-username"        # e.g., "sa"
export SQLSERVER_PASSWORD="your-sql-password"        # Your actual password
export NEW_RELIC_LICENSE_KEY="your-newrelic-key"     # Your New Relic License Key
```

### 2. Run It
```bash
cd testdata/vm-testing
./run.sh
```

**That's it!** ✨

## 🎯 **What Happens**

1. **✅ Environment Check** - Validates all your credentials are set
2. **✅ Collector Start** - Runs OpenTelemetry Collector with your receiver  
3. **✅ Data Collection** - Collects SQL Server metrics every 60 seconds
4. **✅ New Relic Export** - Sends data to New Relic OTLP endpoint

## 📊 **Expected Output**
```
🚀 Running OpenTelemetry Collector with New Relic SQL Server Receiver

✅ Configuration looks good!
SQL Server: 192.168.1.100:1433
Username: monitoring_user
New Relic Key: eu01xxNRAL-your-key...

🎯 Starting OpenTelemetry Collector...
Press Ctrl+C to stop

2025/09/15 15:45:00 info   Metrics {"#metrics": 3}
ResourceMetrics #0
Resource labels:
     -> deployment.environment: STRING(production)
     -> service.name: STRING(sql-server-monitoring)
ScopeMetrics #0
ScopeName: newrelicsqlserverreceiver
Metric #0
Descriptor:
     -> Name: sqlserver.user_connections
     -> DataType: Gauge
     -> Unit: {connections}
NumberDataPoints #0
Data point attributes:
     -> server: STRING(192.168.1.100)
StartTimestamp: 2025-09-15 15:45:00
Timestamp: 2025-09-15 15:45:00
Value: 5
```

## ✅ **Success Indicators**

1. **No Errors** - Script runs without red error messages
2. **Metrics Output** - You see "ResourceMetrics" and "sqlserver.*" metrics  
3. **New Relic Dashboard** - Your metrics appear in New Relic after a few minutes

## 🔧 **Making Changes**

When you update your receiver code:
1. **Edit** your receiver files (factory.go, scraper_*.go, etc.)
2. **Run** `./run.sh` again - changes are included automatically!
3. **No rebuild needed** - Go runs from source with your latest changes

## 🆘 **Troubleshooting**

### Connection Issues
```bash
# Test SQL Server connectivity
telnet your-sql-server-ip 1433

# Check environment variables  
echo $SQLSERVER_HOST
echo $SQLSERVER_USERNAME  
echo $NEW_RELIC_LICENSE_KEY
```

### Debug Mode
Edit `config.yaml` and change:
```yaml
service:
  telemetry:
    logs:
      level: debug  # Change from 'info' to 'debug'
```

### Credential Problems
The script will tell you exactly which environment variables are missing:
```
❌ SQLSERVER_HOST is not set
❌ NEW_RELIC_LICENSE_KEY is not set
```

## 💡 **Why This Works**

- **No OCB Issues** - Bypasses OpenTelemetry Collector Builder version conflicts
- **Live Development** - Uses `go run` so your changes are included immediately  
- **Real Integration** - Uses the actual OpenTelemetry Collector Contrib codebase
- **Production Ready** - Same collector that runs in production environments

## 🎉 **You're All Set!**

This is exactly what you asked for:
1. ✅ **One way to run your local receiver changes** 
2. ✅ **One config with newrelicsqlserverreceiver + New Relic OTLP endpoint**

**Simple workflow:**
```bash
# Set credentials once
export SQLSERVER_HOST="192.168.1.100" 
export SQLSERVER_USERNAME="monitoring_user"
export SQLSERVER_PASSWORD="SecurePass123!"
export NEW_RELIC_LICENSE_KEY="your-actual-key"

# Run anytime
./run.sh
```

Happy monitoring! 🚀📊
