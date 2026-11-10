<?php
/**
 * VoiceOTP Connector - Administrator-only test feature.
 *
 * Capability: manage_options. Nonce-verified. Never exposed to visitors.
 *
 * @package VoiceOTP_Connector
 * @namespace VoiceOTP\Connector
 */

namespace VoiceOTP\Connector;

if ( ! defined( 'ABSPATH' ) ) {
	exit;
}

/**
 * Admin test page for the VoiceOTP Gateway.
 */
class Test {

	public static function register() {
		add_action( 'admin_init', array( __CLASS__, 'handle_test_request' ) );
	}

	public static function handle_test_request() {
		if ( ! isset( $_POST['voiceotp_test_nonce'] ) ) {
			return;
		}
		if ( ! current_user_can( 'manage_options' ) ) {
			wp_die( esc_html__( 'Insufficient permissions.', 'voiceotp-connector' ) );
		}
		if ( ! wp_verify_nonce( sanitize_text_field( wp_unslash( $_POST['voiceotp_test_nonce'] ) ), 'voiceotp_test_action' ) ) {
			wp_die( esc_html__( 'Nonce verification failed.', 'voiceotp-connector' ) );
		}

		$number = sanitize_text_field( wp_unslash( $_POST['voiceotp_test_number'] ?? '' ) );
		$otp    = sanitize_text_field( wp_unslash( $_POST['voiceotp_test_otp'] ?? '' ) );

		if ( empty( $number ) || empty( $otp ) ) {
			add_settings_error( 'voiceotp_test', 'voiceotp_test_empty', __( 'Please enter both destination number and OTP.', 'voiceotp-connector' ), 'error' );
			return;
		}

		$result = Client::send( $number, $otp );

		if ( $result['success'] ) {
			$msg = sprintf(
				esc_html__( 'Test call accepted by gateway (HTTP 202). Request ID: %s', 'voiceotp-connector' ),
				esc_html( $result['data']['request_id'] ?? '' )
			);
			add_settings_error( 'voiceotp_test', 'voiceotp_test_ok', $msg, 'updated' );
		} else {
			$msg = $result['error'] ? $result['error']->get_error_message() : __( 'Unknown error.', 'voiceotp-connector' );
			add_settings_error( 'voiceotp_test', 'voiceotp_test_err', $msg, 'error' );
		}
	}

	public static function render_form() {
		$nonce = wp_create_nonce( 'voiceotp_test_action' );
		?>
		<?php settings_errors( 'voiceotp_test' ); ?>
		<div class="voiceotp-test-wrap">
			<h2><?php esc_html_e( 'Test Gateway Connection', 'voiceotp-connector' ); ?></h2>
			<p><?php esc_html_e( 'Send a test OTP through the configured gateway. This places a real telephone call.', 'voiceotp-connector' ); ?></p>
			<form method="post" action="">
				<?php wp_nonce_field( 'voiceotp_test_action', 'voiceotp_test_nonce' ); ?>
				<table class="form-table">
					<tr>
						<th scope="row"><?php esc_html_e( 'Destination number', 'voiceotp-connector' ); ?></th>
						<td><input type="text" class="regular-text" name="voiceotp_test_number" placeholder="+989121234567" /></td>
					</tr>
					<tr>
						<th scope="row"><?php esc_html_e( 'Test OTP code', 'voiceotp-connector' ); ?></th>
						<td><input type="text" class="regular-text" name="voiceotp_test_otp" placeholder="004829" /></td>
					</tr>
				</table>
				<?php submit_button( esc_html__( 'Send Test OTP', 'voiceotp-connector' ) ); ?>
			</form>
		</div>
		<?php
	}

}
