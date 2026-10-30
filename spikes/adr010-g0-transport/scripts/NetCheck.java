// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import java.net.InetSocketAddress;
import java.net.Socket;

/**
 * Checks whether this JVM can open an outbound TCP connection.
 *
 * <p>The G0 offline runs use it to prove a network block is in force
 * before they run the probe under it. It must connect without the block
 * and fail with it, or the offline run is BLOCKED.
 *
 * <p>Prints {@code CONNECTED <host>} and exits 0, or prints
 * {@code NO-CONNECTION <host>: <reason>} and exits 1.
 */
public final class NetCheck {
    private NetCheck() {}

    public static void main(String[] args) {
        String host = args.length > 0 ? args[0] : "github.com";
        try (Socket s = new Socket()) {
            s.connect(new InetSocketAddress(host, 443), 5000);
            System.out.println("CONNECTED " + host);
        } catch (Exception e) {
            System.out.println("NO-CONNECTION " + host + ": " + e);
            System.exit(1);
        }
    }
}
