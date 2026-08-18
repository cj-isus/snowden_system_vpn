package com.snowden.system.snowden_android

import android.app.Activity
import android.content.Intent
import android.net.VpnService
import android.os.Bundle
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel

class MainActivity : FlutterActivity() {

    private val CHANNEL = "com.snowden.system/vpn"
    private val VPN_REQUEST_CODE = 0x0F
    private var pendingConfig: String? = null

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)

        MethodChannel(flutterEngine.dartExecutor.binaryMessenger, CHANNEL)
            .setMethodCallHandler { call, result ->
                when (call.method) {
                    "startVpn" -> {
                        val config = call.argument<String>("config")
                        if (config != null) {
                            pendingConfig = config
                            if (prepareVpn()) {
                                startVpnService(config)
                                result.success(true)
                            } else {
                                // Will start after user grants permission
                                result.success(false)
                            }
                        } else {
                            result.error("NO_CONFIG", "Config is required", null)
                        }
                    }
                    "stopVpn" -> {
                        stopVpnService()
                        result.success(true)
                    }
                    "getStatus" -> {
                        result.success(SnowdenVpnService.isRunning)
                    }
                    else -> {
                        result.notImplemented()
                    }
                }
            }
    }

    private fun prepareVpn(): Boolean {
        val intent = VpnService.prepare(this)
        if (intent != null) {
            startActivityForResult(intent, VPN_REQUEST_CODE)
            return false
        }
        return true
    }

    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.onActivityResult(requestCode, resultCode, data)
        if (requestCode == VPN_REQUEST_CODE && resultCode == Activity.RESULT_OK) {
            pendingConfig?.let {
                startVpnService(it)
                pendingConfig = null
            }
        }
    }

    private fun startVpnService(config: String) {
        val intent = Intent(this, SnowdenVpnService::class.java).apply {
            action = SnowdenVpnService.ACTION_START
            putExtra(SnowdenVpnService.EXTRA_CONFIG, config)
        }
        startService(intent)
    }

    private fun stopVpnService() {
        val intent = Intent(this, SnowdenVpnService::class.java).apply {
            action = SnowdenVpnService.ACTION_STOP
        }
        startService(intent)
    }
}
