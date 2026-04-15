<?php

class SocketException extends RuntimeException
{
}

class TimeoutException extends RuntimeException
{
}

class ServiceException extends RuntimeException
{
}

class RateLimiterClient
{
	private string $address;
	private int $port;
	private int $connectTimeoutSeconds;
	private int $connectTimeoutMicroseconds;
	private int $receiveTimeoutSeconds;
	private int $receiveTimeoutMicroseconds;
	private int $sendTimeoutSeconds;
	private int $sendTimeoutMicroseconds;
	private int $readBytes;

	private array $errors = [
		'e:internal'        => 'internal service error',
		'e:busy'            => 'too many connections',
		'e:read_timeout'    => 'read timeout, client writing too slow',
		'e:invalid_data'    => 'incorrect incoming message',
		'e:group_not_found' => 'group not found',
	];

	public function __construct(string $address = '127.0.0.1', int $port = 49105, float $timeoutSeconds = 1.0, int $readBytes = 64)
	{
		if ($port < 1 || $port > 65535)
		{
			throw new UnexpectedValueException('invalid port');
		}

		if ($timeoutSeconds <= 0)
		{
			throw new UnexpectedValueException('invalid timeout');
		}

		$timeoutMicroseconds = (int)round($timeoutSeconds * 1000000);
		$timeoutSecondsPart = intdiv($timeoutMicroseconds, 1000000);
		$timeoutMicrosecondsPart = $timeoutMicroseconds % 1000000;

		$this->address = $address;
		$this->port = $port;
		$this->connectTimeoutSeconds = $timeoutSecondsPart;
		$this->connectTimeoutMicroseconds = $timeoutMicrosecondsPart;
		$this->receiveTimeoutSeconds = $timeoutSecondsPart;
		$this->receiveTimeoutMicroseconds = $timeoutMicrosecondsPart;
		$this->sendTimeoutSeconds = $timeoutSecondsPart;
		$this->sendTimeoutMicroseconds = $timeoutMicrosecondsPart;
		$this->readBytes = $readBytes;
	}

	public function send(string $group, string $clientId)
	{
		if ($group === '')
			throw new UnexpectedValueException('empty group');

		if ($clientId === '')
			throw new UnexpectedValueException('empty client id');

		$query = sprintf('%s:%s', $group, $clientId);

var_dump($query);

		$sendTimeout = [
			'sec'  => $this->sendTimeoutSeconds,
			'usec' => $this->sendTimeoutMicroseconds,
		];
		$receiveTimeout = [
			'sec'  => $this->receiveTimeoutSeconds,
			'usec' => $this->receiveTimeoutMicroseconds,
		];

		$socket = socket_create(AF_INET, SOCK_STREAM, SOL_TCP);
		$this->checkSocketError('create socket', $socket);

		try
		{
			socket_set_option($socket, SOL_SOCKET, SO_SNDTIMEO, $sendTimeout);
			$this->checkSocketError('set send timeout', $socket);

			socket_set_option($socket, SOL_SOCKET, SO_RCVTIMEO, $receiveTimeout);
			$this->checkSocketError('set receive timeout', $socket);

			socket_set_nonblock($socket);
			$this->checkSocketError('socket set nonblock', $socket);

			$connected = @socket_connect($socket, $this->address, $this->port);
			if ($connected === false)
			{
				$error = socket_last_error($socket);

				if (!in_array($error, [SOCKET_EINPROGRESS, SOCKET_EALREADY], true))
				{
					$this->checkSocketError('connection error', $socket);
				}

				$read = null;
				$write = [$socket];
				$except = [$socket];

				$select = socket_select($read, $write, $except, $this->connectTimeoutSeconds, $this->connectTimeoutMicroseconds);
				if ($select === 0)
				{
					throw new TimeoutException(
						"connection timeout after {$this->connectTimeoutSeconds}s {$this->connectTimeoutMicroseconds}us"
					);
				}

				if ($select === false)
				{
					$this->checkSocketError('socket select error', $socket);
				}

				$errorCode = socket_get_option($socket, SOL_SOCKET, SO_ERROR);
				if ($errorCode !== 0)
				{
					throw new SocketException(
						'socket connection status error: ' . socket_strerror($errorCode),
						$errorCode
					);
				}
			}

			socket_set_block($socket);
			$this->checkSocketError('socket set block', $socket);

			$written = socket_write($socket, $query, strlen($query));
			if ($written === false || $written !== strlen($query))
			{
				$this->checkSocketError('socket write', $socket);
				throw new SocketException('partial write');
			}

			socket_shutdown($socket, 1);
			$this->checkSocketError('shutdown socket writing', $socket);

			$response = socket_read($socket, $this->readBytes);
			$this->checkSocketError('reading response', $socket);

			if ($response === false || $response === '')
			{
				throw new UnexpectedValueException('empty response');
			}

			$response = trim($response);
			$this->checkResponse($response);

			return $response;
		}
		finally
		{
			if (is_object($socket))
			{
				socket_close($socket);
			}
		}
	}

	private function checkResponse(string $response)
	{
		if (array_key_exists($response, $this->errors))
		{
			throw new ServiceException($this->errors[$response]);
		}
	}

	private function checkSocketError(string $message, $socket)
	{
		if ($socket === false)
		{
			$error = socket_last_error();
			throw new SocketException($message . ': ' . socket_strerror($error), $error);
		}

		$error = socket_last_error($socket);
		if ($error === 0)
			return;

		if (in_array($error, [SOCKET_EINPROGRESS, SOCKET_EALREADY], true))
			return;

		$errorMessage = socket_strerror($error);
		socket_clear_error($socket);

		throw new SocketException($message . ': ' . $errorMessage, $error);
	}
}

function parseCliOptions(): array
{
	$options = getopt('', ['host::', 'port::', 'group::', 'client-id::', 'timeout::']);

	return [
		'host'      => isset($options['host']) && $options['host'] !== false ? (string)$options['host'] : '127.0.0.1',
		'port'      => isset($options['port']) && $options['port'] !== false ? (int)$options['port'] : 49105,
		'group'     => isset($options['group']) && $options['group'] !== false ? (string)$options['group'] : 'default',
		'client_id' => isset($options['client-id']) && $options['client-id'] !== false ? (string)$options['client-id'] : 'token_value',
		'timeout'   => isset($options['timeout']) && $options['timeout'] !== false ? (float)$options['timeout'] : 1.0,
	];
}

try
{
	$options = parseCliOptions();

	var_dump($options);

	$client = new RateLimiterClient($options['host'], $options['port'], $options['timeout']);
	$response = $client->send($options['group'], $options['client_id']);

	echo $response . PHP_EOL;
}
catch (Exception $e)
{
	fwrite(STDERR, 'error: ' . $e->getMessage() . PHP_EOL);
	exit(1);
}
