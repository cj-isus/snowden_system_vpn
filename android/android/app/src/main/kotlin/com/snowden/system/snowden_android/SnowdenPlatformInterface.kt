package com.snowden.system.snowden_android

import android.content.Context
import android.net.ConnectivityManager
import android.net.NetworkCapabilities
import android.net.VpnService
import android.os.ParcelFileDescriptor
import io.nekohasekai.libbox.BridgeOptions
import io.nekohasekai.libbox.BridgeSession
import io.nekohasekai.libbox.ConnectionOwner
import io.nekohasekai.libbox.Libbox
import io.nekohasekai.libbox.LocalDNSTransport
import io.nekohasekai.libbox.NetworkInterfaceIterator
import io.nekohasekai.libbox.Notification
import io.nekohasekai.libbox.PlatformInterface
import io.nekohasekai.libbox.PlatformUser
import io.nekohasekai.libbox.InterfaceUpdateListener
import io.nekohasekai.libbox.NeighborUpdateListener
import io.nekohasekai.libbox.ShellSession
import io.nekohasekai.libbox.StringIterator
import io.nekohasekai.libbox.TunOptions
import io.nekohasekai.libbox.WIFIState
import java.io.File

/**
 * Adapter between libbox v1.14.0-lx.3 and Android.
 *
 * The libbox PlatformInterface contract was completely rewritten compared
 * to the previous generation (sing-box 1.11.x): openTun returns Int,
 * autoDetectInterfaceControl is now void, usePlatformAutoDetectInterface
 * was renamed to usePlatformAutoDetectInterfaceControl, underVPN was
 * renamed to underNetworkExtension, and the package/uid getters were
 * replaced by a SSH/SFTP-oriented lookupUser/openShellSession scope.
 *
 * Scope we don't need (SSH/SFTP/Tailscale/USB/Neighbor monitor) returns
 * safe defaults or throws UnsupportedOperationException, so the SDK
 * keeps functioning but the relevant features are explicitly disabled.
 */
class SnowdenPlatformInterface(
    private val context: Context,
    private val vpnService: VpnService
) : PlatformInterface {

    private var currentTunFd: ParcelFileDescriptor? = null

    // -- TUN ----------------------------------------------------------------

    override fun openTun(options: TunOptions): Int {
        try {
            // Close existing interface if any
            currentTunFd?.close()
            currentTunFd = null

            val builder = vpnService.Builder()
                .setSession("snowden.system")
                .setMtu(options.mtu.toInt())

            // DNS configuration if provided
            try {
                val dnsItr: StringIterator = options.dnsServerAddress
                while (dnsItr.hasNext()) {
                    builder.addDnsServer(dnsItr.next())
                }
            } catch (_: Exception) {
                // No DNS servers in this profile - fall back to safe public ones.
                builder.addDnsServer("1.1.1.1")
                builder.addDnsServer("8.8.8.8")
            }

            // IPv4 routes - default for now (steered by libbox dns routing)
            builder.addAddress("172.19.0.1", 30)
            builder.addRoute("0.0.0.0", 0)

            // IPv6 routes if provided
            try {
                val v6 = options.inet6RouteAddress
                if (v6.hasNext()) {
                    builder.addRoute("::", 0)
                }
            } catch (_: Exception) {
                // No IPv6 routes - skip.
            }

            // Exclude our own app to avoid routing loop
            try {
                val excluded: StringIterator = options.excludePackage
                while (excluded.hasNext()) {
                    builder.addDisallowedApplication(excluded.next())
                }
            } catch (_: Exception) {
                builder.addDisallowedApplication(context.packageName)
            }

            val fd = builder.establish()
                ?: throw Exception("VpnService.Builder.establish() returned null")

            currentTunFd = fd
            return fd.fd
        } catch (e: Exception) {
            e.printStackTrace()
            return -1
        }
    }

    // -- Network probes ----------------------------------------------------

    override fun autoDetectInterfaceControl(fd: Int) {
        // No-op: we don't do per-fd interface control on Android.
        // The fd is owned by VpnService and libbox reads/writes it directly.
    }

    override fun findConnectionOwner(
        ipProtocol: Int,
        sourceAddress: String,
        sourcePort: Int,
        destinationAddress: String,
        destinationPort: Int
    ): ConnectionOwner {
        // Linux-style /proc/net/tcp lookup could go here, but on Android
        // the system already resolves ownership through connectivityManager.
        // Return an empty owner; libbox treats this as "no constraint".
        return ConnectionOwner().apply {
            userId = 0
            userName = ""
            processPath = ""
        }
    }

    override fun getInterfaces(): NetworkInterfaceIterator {
        // MVP: return an empty iterator. android.net.NetworkCapabilities can
        // be iterated later to enumerate real interfaces when needed.
        return object : NetworkInterfaceIterator {
            override fun hasNext(): Boolean = false
            override fun next(): io.nekohasekai.libbox.NetworkInterface {
                throw NoSuchElementException("no interfaces")
            }
        }
    }

    override fun includeAllNetworks(): Boolean = false

    override fun localDNSTransport(): LocalDNSTransport? = null

    // -- Capability flags --------------------------------------------------

    override fun useProcFS(): Boolean = false

    override fun usePlatformAutoDetectInterfaceControl(): Boolean = false

    override fun usePlatformBridge(): Boolean = false

    override fun usePlatformShell(): Boolean = false

    override fun underNetworkExtension(): Boolean = true

    override fun tailscaleHostname(): String = ""

    // -- WIFI --------------------------------------------------------------

    override fun readWIFIState(): WIFIState {
        val cm = context.getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager
        val network = cm.activeNetwork
        val capabilities = cm.getNetworkCapabilities(network)
        val onWifi =
            capabilities != null && capabilities.hasTransport(NetworkCapabilities.TRANSPORT_WIFI)
        return if (onWifi) {
            Libbox.newWIFIState("WIFI", "")
        } else {
            Libbox.newWIFIState("", "")
        }
    }

    override fun clearDNSCache() {
        // No-op: Android's netd flushes DNS via VpnService.Builder automatically.
    }

    // -- Default interface / Neighbor monitors -----------------------------

    override fun startDefaultInterfaceMonitor(listener: InterfaceUpdateListener) {
        // No-op: we don't expose default-interface changes to libbox.
    }

    override fun closeDefaultInterfaceMonitor(listener: InterfaceUpdateListener) {
        // No-op.
    }

    override fun startNeighborMonitor(listener: NeighborUpdateListener) {
        // No-op: we don't expose neighbor table to libbox.
    }

    override fun closeNeighborMonitor(listener: NeighborUpdateListener) {
        // No-op.
    }

    // -- Notifications -----------------------------------------------------

    override fun sendNotification(notification: Notification) {
        // libbox notifications are turned into SystemProxyStatus messages
        // (handled by CommandServerHandler). We don't relay them to the
        // Android NotificationManager here to avoid duplicate notification
        // icons alongside the VpnService foreground notification.
    }

    override fun registerMyInterface(name: String) {
        // Used by libbox on platforms where creating a TAP-style peer
        // interface requires registering it with the kernel. Android has
        // VpnService for that, so nothing to do.
    }

    // -- SSH / SFTP / Bridges (out-of-MVP scope) --------------------------

    override fun lookupUser(username: String): PlatformUser? {
        throw UnsupportedOperationException("SSH user lookup is out of MVP scope")
    }

    override fun openShellSession(
        user: PlatformUser,
        command: String,
        environment: StringIterator,
        arguments: String,
        cols: Int,
        rows: Int
    ): ShellSession? {
        throw UnsupportedOperationException("Shell sessions are out of MVP scope")
    }

    override fun lookupSFTPServer(): String = ""

    override fun readSystemSSHHostKey(): String = ""

    override fun createBridge(options: BridgeOptions): BridgeSession? {
        throw UnsupportedOperationException("Bridge devices are out of MVP scope")
    }

    override fun checkPlatformShell() {
        throw UnsupportedOperationException("checkPlatformShell is out of MVP scope")
    }
}
