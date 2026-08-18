package com.snowden.system.snowden_android

import android.content.Context
import android.net.ConnectivityManager
import android.net.NetworkCapabilities
import android.net.VpnService
import android.os.ParcelFileDescriptor
import io.nekohasekai.libbox.Libbox
import io.nekohasekai.libbox.PlatformInterface
import io.nekohasekai.libbox.StringBox
import io.nekohasekai.libbox.TunOptions
import java.io.File

class SnowdenPlatformInterface(
    private val context: Context,
    private val vpnService: VpnService
) : PlatformInterface {

    private var currentTunFd: ParcelFileDescriptor? = null

    /**
     * openTun is called by libbox to create the TUN interface.
     * We build the VPN tunnel using VpnService.Builder, establish it,
     * and return the file descriptor to libbox.
     */
    override fun openTun(options: TunOptions): Long {
        try {
            // Close existing interface if any
            currentTunFd?.close()
            currentTunFd = null

            val builder = vpnService.Builder()
                .setSession("snowden.system")
                .setMtu(options.mtu.toInt())
                .addAddress("172.19.0.1", 30)
                .addDnsServer("1.1.1.1")
                .addDnsServer("8.8.8.8")
                .addRoute("0.0.0.0", 0)
                .addRoute("::", 0)

            // Exclude our own app to avoid routing loop
            builder.addDisallowedApplication(context.packageName)

            // Establish the VPN interface
            val fd = builder.establish()
            if (fd == null) {
                throw Exception("VpnService.Builder.establish() returned null")
            }

            currentTunFd = fd
            return fd.fd.toLong()

        } catch (e: Exception) {
            e.printStackTrace()
            return -1
        }
    }

    override fun closeTun(fd: Long) {
        try {
            currentTunFd?.close()
            currentTunFd = null
        } catch (e: Exception) {
            e.printStackTrace()
        }
    }

    override fun autoDetectInterfaceControl(fd: Long): Long {
        return fd
    }

    override fun useProcFS(): Boolean {
        return false
    }

    override fun usePlatformAutoDetectInterface(): Boolean {
        return true
    }

    override fun useWIFIState(): Boolean {
        return true
    }

    override fun underVPN(): Boolean {
        return true
    }

    override fun clearDNSCache() {
        // No-op
    }

    override fun readWIFIState(): StringBox {
        val cm = context.getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager
        val network = cm.activeNetwork
        val capabilities = cm.getNetworkCapabilities(network)

        val ssid = if (capabilities != null && capabilities.hasTransport(NetworkCapabilities.TRANSPORT_WIFI)) {
            "WIFI"
        } else {
            "MOBILE"
        }

        return Libbox.newStringBox(ssid)
    }

    override fun findConnectionOwner(
        ipProtocol: Long,
        sourceAddress: String,
        sourcePort: Long,
        destinationAddress: String,
        destinationPort: Long
    ): Long {
        return -1
    }

    override fun packageNameByUid(uid: Long): String {
        return ""
    }

    override fun uidByPackageName(packageName: String): Long {
        return -1
    }

    override fun useGetter(): Boolean {
        return false
    }

    override fun getSystemProxyStatus(): Long {
        return 0
    }

    override fun setSystemProxyEnabled(enabled: Boolean) {
        // No-op on Android
    }

    override fun getInterfaces(): StringBox {
        return Libbox.newStringBox("tun0")
    }

    override fun getPackageName(): String {
        return context.packageName
    }

    override fun getUserID(): Long {
        return android.os.Process.myUid().toLong()
    }

    override fun getGroupID(): Long {
        return android.os.Process.myUid().toLong()
    }
}
