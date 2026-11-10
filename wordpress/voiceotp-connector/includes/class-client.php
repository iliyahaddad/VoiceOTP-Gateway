<?php
/**
 * VoiceOTP Connector - HTTP client to the VoiceOTP Gateway.
 *
 * Developer integration interface:
 *
 *   $result = \VoiceOTP\Connector\Client::send( '+989121234567', '004829' );
 *
 * @package VoiceOTP_Connector
 * @namespace VoiceOTP\Connector
 */

namespace VoiceOTP\Connector;

use WP_Error;
use WP_Http;

/**
 * Client for the VoiceOTP Gateway HTTP API.
 */
class Client {

	const ENDPOINT_PATH = '/api/v1/voice-otp';

	// Error codes.
	const ERROR_NETWORK      = 'http_request_failed';
	const ERROR_TIMEOUT      = 'request_timeout';
	const ERROR_TLS          = 'tls_certificate_error';
	const ERROR_INVALID_JSON = 'invalid_json_response';
	const ERROR_INVALID_RESP = 'invalid_response';
	const ERROR_AUTH         = 'authentication_failed';
	const ERROR_VALIDATION   = 'validation_failed';
	const ERROR_RATE_LIMIT   = 'rate_limit_exceeded';
	const ERROR_GATEWAY      = 'gateway_error';

	private static $instance = null;

	public static function instance() {
		if ( null === self::$instance ) {
			self::$instance = new self();
		}
		return self::$instance;
	}

	/**
	 * Send an OTP voice call.
	 *
	 * @param string $number  E.164 telephone number, e.g. +989121234567.
	 * @param string $otp     OTP digit string (leading zeros preserved).
	 * @return array{success: bool, status_code: int|null, data: array|null, error: WP_Error|null}
	 */
	public static function send( $number, $otp ) {
		return self::instance()->send_otp( $number, $otp );
	}

	/**
	 * Send an OTP voice call with explicit credentials.
	 *
	 * @param string $number   E.164 telephone number.
	 * @param string $otp      OTP digit string.
	 * @param string $username API username.
	 * @param string $password API password.
	 * @return array{success: bool, status_code: int|null, data: array|null, error: WP_Error|null}
	 */
	public static function send_with_credentials( $number, $otp, $username, $password ) {
		return self::instance()->send_otp( $number, $otp, $username, $password );
	}

	private function send_otp( $number, $otp, $username = null, $password = null ) {
		$settings = $this->get_settings();
		if ( empty( $settings['enabled'] ) ) {
			return array(
				'success'    => false,
				'status_code'=> null,
				'data'       => null,
				'error'      => new WP_Error( 'connector_disabled', __( 'VoiceOTP connector is disabled.', 'voiceotp-connector' ) ),
			);
		}

		$gateway_url = trailingslashit( $settings['gateway_url'] );
		$username    = $username ?: $settings['username'];
		$password    = $password ?: $settings['password'];
		$timeout     = (int) $settings['timeout'] ?: 10;

		$body = array(
			'username' => $username,
			'password' => $password,
			'otp'      => $otp,
			'number'   => $number,
		);

		$response = wp_remote_post(
			$gateway_url . self::ENDPOINT_PATH,
			array(
				'method'      => 'POST',
				'timeout'     => $timeout,
				'redirection' => 5,
				'httpversion' => '1.1',
				'blocking'    => true,
				'headers'     => array(
					'Content-Type'  => 'application/json',
					'Accept'        => 'application/json',
					'Idempotency-Key' => 'voiceotp-wp-' . md5( $number . '|' . $otp . '|' . $username ),
				),
				'body'        => wp_json_encode( $body ),
				'sslverify'   => ! empty( $settings['verify_ssl'] ),
				'sslcertificates' => abspath() . 'certs', // not used; sslverify controls verification.
			)
		);

		// Never log the full request body or credentials.
		if ( is_wp_error( $response ) ) {
			$error = $response;
			$code  = $error->get_error_code();
			if ( strpos( $code, 'http_request_failed' ) !== false && strpos( $error->get_error_message(), 'timed out' ) !== false ) {
				$code = self::ERROR_TIMEOUT;
			}
			if ( strpos( $error->get_error_message(), 'certificate' ) !== false || strpos( $error->get_error_message(), 'SSL' ) !== false ) {
				$code = self::ERROR_TLS;
			}
			return array(
				'success'    => false,
				'status_code'=> null,
				'data'       => null,
				'error'      => new WP_Error( $code, $error->get_error_message() ),
			);
		}

		$status_code = wp_remote_retrieve_response_code( $response );
		$body_text   = wp_remote_retrieve_body( $response );

		$data = json_decode( $body_text, true );
		if ( ! is_array( $data ) ) {
			return array(
				'success'    => false,
				'status_code'=> $status_code,
				'data'       => null,
				'error'      => new WP_Error( self::ERROR_INVALID_JSON, __( 'Invalid JSON response from gateway.', 'voiceotp-connector' ) ),
			);
		}

		switch ( $status_code ) {
			case 202:
				return array(
					'success'    => true,
					'status_code'=> 202,
					'data'       => $data,
					'error'      => null,
				);
			case 401:
				return array(
					'success'    => false,
					'status_code'=> 401,
					'data'       => $data,
					'error'      => new WP_Error( self::ERROR_AUTH, __( 'Gateway authentication failed.', 'voiceotp-connector' ) ),
				);
			case 400:
				return array(
					'success'    => false,
					'status_code'=> 400,
					'data'       => $data,
					'error'      => new WP_Error( self::ERROR_VALIDATION, __( 'Invalid OTP or telephone number.', 'voiceotp-connector' ) ),
				);
			case 429:
				return array(
					'success'    => false,
					'status_code'=> 429,
					'data'       => $data,
					'error'      => new WP_Error( self::ERROR_RATE_LIMIT, __( 'Gateway rate limit exceeded.', 'voiceotp-connector' ) ),
				);
			case 502:
			case 503:
				return array(
					'success'    => false,
					'status_code'=> $status_code,
					'data'       => $data,
					'error'      => new WP_Error( self::ERROR_GATEWAY, __( 'Gateway error.', 'voiceotp-connector' ) ),
				);
			default:
				return array(
					'success'    => false,
					'status_code'=> $status_code,
					'data'       => $data,
					'error'      => new WP_Error( self::ERROR_INVALID_RESP, __( 'Unexpected response from gateway.', 'voiceotp-connector' ) ),
				);
		}
	}

	private function get_settings() {
		return array(
			'enabled'      => get_option( 'voiceotp_enabled', '0' ),
			'gateway_url'  => esc_url_raw( get_option( 'voiceotp_gateway_url', '' ) ),
			'username'     => sanitize_text_field( get_option( 'voiceotp_username', '' ) ),
			'password'     => get_option( 'voiceotp_password', '' ),
			'timeout'      => absint( get_option( 'voiceotp_timeout', 10 ) ),
			'verify_ssl'   => get_option( 'voiceotp_verify_ssl', '1' ),
		);
	}

}

/**
 * Convenience wrapper for the documented developer interface.
 *
 * @param string $number E.164 telephone number.
 * @param string $otp    OTP digit string.
 * @return array{success: bool, status_code: int|null, data: array|null, error: WP_Error|null}
 */
function voiceotp_send_otp( $number, $otp ) {
	return Client::send( $number, $otp );
}
