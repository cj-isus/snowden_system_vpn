package com.snowden.system.snowden_android

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Intent
import android.net.VpnService
import android.os.Build
import android.os.IBinder
import android.os.ParcelFileDescriptor
import androidx.core.app.NotificationCompat
import io.nekohasekai.libbox.Libbox
import io.nekohasekai.libbox.PlatformInterface
import java.io.File

class SnowdenVpnService : VpnService() {

    companion object {
        const val ACTION_START = "com.snowden.system.START_VPN"
        const val ACTION_STOP = "com.snowden.system.STOP_VPN"
        const val EXTRA_CONFIG = "config"
        const val NOTIFICATION_CHANNEL_ID = "snowden_vpn"
        const val NOTIFICATION_ID = 1

        @JvmStatic
        var isRunning = false
    }

    private var platformInterface: SnowdenPlatformInterface? = null
    private var serviceFile: File? = null

    override fun onCreate() {
        super.onCreate()
        createNotificationChannel()
    }

    override fun onBind(intent: Intent?): IBinder? {
        return null
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_START -> {
                val config = intent.getStringExtra(EXTRA_CONFIG) ?: return START_NOT_STICKY
                startVpn(config)
            }
            ACTION_STOP -> {
                stopVpn()
            }
        }
        return START_NOT_STICKY
    }

    private fun startVpn(config: String) {
        try {
            // Write config to temp file for libbox
            serviceFile = File(cacheDir, "snowden-config.json")
            serviceFile?.writeText(config)

            // Create platform interface with this VpnService
            platformInterface = SnowdenPlatformInterface(this, this)

            // Setup libbox with our platform interface
            Libbox.setup(platformInterface!!)

            // Start libbox service — it will call openTun() via PlatformInterface
            Libbox.start(serviceFile!!.absolutePath)

            isRunning = true
            startForeground(NOTIFICATION_ID, buildNotification("snowden.system — VPN активен"))

        } catch (e: Exception) {
            e.printStackTrace()
            stopVpn()
        }
    }

    private fun stopVpn() {
        try {
            Libbox.stop()
        } catch (e: Exception) {
            e.printStackTrace()
        }

        platformInterface = null
        serviceFile?.delete()
        serviceFile = null

        isRunning = false
        stopForeground(STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    override fun onDestroy() {
        stopVpn()
        super.onDestroy()
    }

    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                NOTIFICATION_CHANNEL_ID,
                "Snowden VPN",
                NotificationManager.IMPORTANCE_LOW
            ).apply {
                description = "VPN connection status"
            }
            val notificationManager = getSystemService(NotificationManager::class.java)
            notificationManager.createNotificationChannel(channel)
        }
    }

    private fun buildNotification(text: String): Notification {
        val intent = Intent(this, MainActivity::class.java).apply {
            flags = Intent.FLAG_ACTIVITY_CLEAR_TOP or Intent.FLAG_ACTIVITY_SINGLE_TOP
        }
        val pendingIntent = PendingIntent.getActivity(
            this, 0, intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )

        return NotificationCompat.Builder(this, NOTIFICATION_CHANNEL_ID)
            .setContentTitle("snowden.system")
            .setContentText(text)
            .setSmallIcon(android.R.drawable.ic_lock_idle_lock)
            .setContentIntent(pendingIntent)
            .setOngoing(true)
            .build()
    }
}
