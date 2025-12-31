package re.mkw.srejaas;

import ghidra.server.remote.GhidraServer;
import io.grpc.*;
import org.apache.logging.log4j.LogManager;
import org.apache.logging.log4j.Logger;

import java.io.IOException;
import java.net.InetSocketAddress;
import java.net.SocketAddress;
import java.util.ArrayList;
import java.util.concurrent.TimeUnit;

public class PanelServer {
  private final Logger log;
  private final SocketAddress address;
  private final Server server;

  public static void main(String[] args) throws InterruptedException, IOException {
    // Parse arguments
    SocketAddress address = new InetSocketAddress(13103);
    ArrayList<String> argList = new ArrayList<>();
    int i = 0;
    while (i < args.length) {
      if (args[i].equals("-grpc-port") && i + 1 < args.length) {
        address = new InetSocketAddress(Integer.parseInt(args[i + 1]));
        i += 2;
      } else {
        argList.add(args[i]);
        i++;
      }
    }

    // Start Ghidra server in a separate thread
    final SocketAddress finalAddress = address;
    Thread ghidraThread = new Thread(() -> {
      try {
        GhidraServer.main(argList.toArray(String[]::new));
      } catch (Exception e) {
        e.printStackTrace();
        System.exit(1);
      }
    }, "Ghidra-Server");
    ghidraThread.start();

    // Wait for GhidraServer to initialize (poll until server instance is available)
    int maxWaitSeconds = 30;
    int waitCount = 0;
    while (waitCount < maxWaitSeconds * 10) {
      try {
        if (re.mkw.srejaas.reflect.GhidraServerSupport.getGhidraServer() != null) {
          break;
        }
      } catch (Exception ignored) {
      }
      Thread.sleep(100);
      waitCount++;
    }

    if (waitCount >= maxWaitSeconds * 10) {
      System.err.println("*** Failed to initialize Ghidra server within " + maxWaitSeconds + " seconds");
      System.exit(1);
    }

    // Now start gRPC server
    final PanelServer grpcServer = new PanelServer(finalAddress);
    grpcServer.start();

    // Wait for Ghidra thread to finish
    ghidraThread.join();
  }

  public PanelServer(SocketAddress address) {
    this.log = LogManager.getLogger(PanelServer.class);
    this.address = address;
    this.server = ServerBuilder.forPort(((InetSocketAddress) address).getPort())
        .addService(new GrpcImpl())
        .build();
  }

  private void start() throws IOException {
    server.start();
    log.info("gRPC server started, listening on {}", this.address);
    Runtime.getRuntime().addShutdownHook(new Thread(() -> {
      // Use stderr here since the logger may have been reset by its JVM shutdown hook.
      System.err.println("*** shutting down gRPC server since JVM is shutting down");
      try {
        PanelServer.this.stop();
      } catch (InterruptedException e) {
        e.printStackTrace(System.err);
      }
      System.err.println("*** server shut down");
    }));
  }

  private void stop() throws InterruptedException {
    if (server != null) {
      server.shutdown().awaitTermination(30, TimeUnit.SECONDS);
    }
  }

  private void blockUntilShutdown() throws InterruptedException {
    if (server != null) {
      server.awaitTermination();
    }
  }
}
