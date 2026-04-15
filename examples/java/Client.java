import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.net.Socket;
import java.nio.charset.StandardCharsets;

public final class Client {
	private static final String DEFAULT_HOST = "127.0.0.1";
	private static final int DEFAULT_PORT = 49105;
	private static final String DEFAULT_GROUP = "default";
	private static final String DEFAULT_CLIENT_ID = "token_value";
	private static final int DEFAULT_TIMEOUT_MS = 1000;

	private Client() {
	}

	public static void main(String[] args) {
		Config config = parseArgs(args);

		String request = config.group + ":" + config.clientId;

		try {
			String response = sendRequest(config.host, config.port, request, config.timeoutMs);
			System.out.println(response);
		} catch (IOException e) {
			System.err.println("error: " + e.getMessage());
			System.exit(1);
		}
	}

	private static Config parseArgs(String[] args) {
		Config config = new Config();

		for (int i = 0; i < args.length; i++) {
			String arg = args[i];
			if ("--help".equals(arg)) {
				printUsageAndExit("");
			} else if ("--host".equals(arg)) {
				config.host = requireValue(args, ++i, "--host");
			} else if ("--port".equals(arg)) {
				config.port = Integer.parseInt(requireValue(args, ++i, "--port"));
			} else if ("--group".equals(arg)) {
				config.group = requireValue(args, ++i, "--group");
			} else if ("--client-id".equals(arg)) {
				config.clientId = requireValue(args, ++i, "--client-id");
			} else if ("--timeout".equals(arg)) {
				config.timeoutMs = Integer.parseInt(requireValue(args, ++i, "--timeout"));
			} else {
				printUsageAndExit("unknown argument: " + arg);
			}
		}

		return config;
	}

	private static String requireValue(String[] args, int index, String option) {
		if (index >= args.length) {
			printUsageAndExit("missing value for " + option);
		}

		return args[index];
	}

	private static void printUsageAndExit(String message) {

		if (message != ""){
			System.err.println(message);
		}
		System.err.println("usage: java Client [--host HOST] [--port PORT] [--group GROUP] [--client-id CLIENT_ID] [--timeout TIMEOUT_MS]");
		System.exit(1);
	}

	private static String sendRequest(String host, int port, String request, int timeoutMs) throws IOException {
		byte[] payload = request.getBytes(StandardCharsets.UTF_8);

		try (Socket socket = new Socket()) {
			socket.connect(new InetSocketAddress(host, port), timeoutMs);
			socket.setSoTimeout(timeoutMs);

			OutputStream output = socket.getOutputStream();
			output.write(payload);
			output.flush();
			socket.shutdownOutput();

			String response = readResponse(socket.getInputStream()).trim();
			if (response.isEmpty()) {
				throw new IOException("empty response from server");
			}

			return response;
		}
	}

	private static String readResponse(InputStream input) throws IOException {
		byte[] buffer = new byte[4096];
		ByteArrayOutputStream response = new ByteArrayOutputStream();

		int read;
		while ((read = input.read(buffer)) != -1) {
			response.write(buffer, 0, read);
		}

		return response.toString(StandardCharsets.UTF_8);
	}

	private static final class Config {
		private String host = DEFAULT_HOST;
		private int port = DEFAULT_PORT;
		private String group = DEFAULT_GROUP;
		private String clientId = DEFAULT_CLIENT_ID;
		private int timeoutMs = DEFAULT_TIMEOUT_MS;
	}
}
