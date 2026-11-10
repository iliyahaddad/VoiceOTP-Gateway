<?php
/**
 * VoiceOTP Connector - Administration settings page.
 *
 * Uses the WordPress Settings API. Requires manage_options capability.
 * Nonces are verified on all administrative actions.
 *
 * @package VoiceOTP_Connector
 * @namespace VoiceOTP\Connector
 */

namespace VoiceOTP\Connector;

use WP_Error;

if ( ! defined( 'ABSPATH' ) ) {
	exit;
}

/**
 * Settings page for VoiceOTP Connector.
 */
class Settings {

	const OPTION_GROUP = 'voiceotp_settings';

	public static function register() {
		register_setting(
			self::OPTION_GROUP,
			'voiceotp_enabled',
			array( 'type' => 'string', 'sanitize_callback' => __CLASS__ . '::sanitize_enabled' )
		);
		register_setting(
			self::OPTION_GROUP,
			'voiceotp_gateway_url',
			array( 'type' => 'string', 'sanitize_callback' => __CLASS__ . '::sanitize_url' )
		);
		register_setting(
			self::OPTION_GROUP,
			'voiceotp_username',
			array( 'type' => 'string', 'sanitize_callback' => 'sanitize_text_field' )
		);
		register_setting(
			self::OPTION_GROUP,
			'voiceotp_password',
			array( 'type' => 'string', 'sanitize_callback' => __CLASS__ . '::sanitize_password' )
		);
		register_setting(
			self::OPTION_GROUP,
			'voiceotp_timeout',
			array( 'type' => 'integer', 'sanitize_callback' => 'absint' )
		);
		register_setting(
			self::OPTION_GROUP,
			'voiceotp_verify_ssl',
			array( 'type' => 'string', 'sanitize_callback' => __CLASS__ . '::sanitize_enabled' )
		);

		add_settings_section(
			'voiceotp_section_main',
			__( 'Gateway Connection', 'voiceotp-connector' ),
			array( __CLASS__, 'render_section_main' ),
			'voiceotp-connector'
		);

		add_settings_field(
			'voiceotp_enabled',
			__( 'Enable connector', 'voiceotp-connector' ),
			array( __CLASS__, 'render_enabled' ),
			'voiceotp-connector',
			'voiceotp_section_main'
		);
		add_settings_field(
			'voiceotp_gateway_url',
			__( 'Gateway HTTPS URL', 'voiceotp-connector' ),
			array( __CLASS__, 'render_gateway_url' ),
			'voiceotp-connector',
			'voiceotp_section_main'
		);
		add_settings_field(
			'voiceotp_username',
			__( 'API username', 'voiceotp-connector' ),
			array( __CLASS__, 'render_username' ),
			'voiceotp-connector',
			'voiceotp_section_main'
		);
		add_settings_field(
			'voiceotp_password',
			__( 'API password', 'voiceotp-connector' ),
			array( __CLASS__, 'render_password' ),
			'voiceotp-connector',
			'voiceotp_section_main'
		);
		add_settings_field(
			'voiceotp_timeout',
			__( 'Connection timeout (seconds)', 'voiceotp-connector' ),
			array( __CLASS__, 'render_timeout' ),
			'voiceotp-connector',
			'voiceotp_section_main'
		);
		add_settings_field(
			'voiceotp_verify_ssl',
			__( 'Verify TLS certificates', 'voiceotp-connector' ),
			array( __CLASS__, 'render_verify_ssl' ),
			'voiceotp-connector',
			'voiceotp_section_main'
		);
	}

	public static function init_defaults() {
		if ( false === get_option( 'voiceotp_enabled' ) ) {
			add_option( 'voiceotp_enabled', '0' );
		}
		if ( false === get_option( 'voiceotp_verify_ssl' ) ) {
			add_option( 'voiceotp_verify_ssl', '1' );
		}
		if ( false === get_option( 'voiceotp_timeout' ) ) {
			add_option( 'voiceotp_timeout', '10' );
		}
	}

	public static function render_page() {
		if ( ! current_user_can( 'manage_options' ) ) {
			return;
		}

		if ( isset( $_GET['settings-updated'] ) && $_GET['settings-updated'] === 'true' ) {
			echo '<div class="notice notice-success is-dismissible"><p>' .
				esc_html__( 'VoiceOTP settings saved.', 'voiceotp-connector' ) . '</p></div>';
		}

		settings_errors( 'voiceotp_settings' );

		?>
		<div class="wrap">
			<h1><?php esc_html_e( 'VoiceOTP Connector Settings', 'voiceotp-connector' ); ?></h1>
			<p><?php esc_html_e( 'Configure the connection to the VoiceOTP Gateway. Credentials are stored in the WordPress options table and are never exposed to the browser.', 'voiceotp-connector' ); ?></p>
			<form method="post" action="options.php">
				<?php
				settings_fields( self::OPTION_GROUP );
				do_settings_sections( 'voiceotp-connector' );
				submit_button();
				?>
			</form>
		</div>
		<?php
	}

	public static function render_section_main() {
		esc_html_e( 'Connection details for the VoiceOTP Gateway instance.', 'voiceotp-connector' );
	}

	public static function render_enabled() {
		$value = get_option( 'voiceotp_enabled', '0' );
		?>
		<input type="checkbox" name="voiceotp_enabled" value="1"<?php checked( $value, '1' ); ?> />
		<span class="description"><?php esc_html_e( 'Enable sending OTPs through the VoiceOTP Gateway.', 'voiceotp-connector' ); ?></span>
		<?php
	}

	public static function render_gateway_url() {
		$value = esc_url_raw( get_option( 'voiceotp_gateway_url', '' ) );
		?>
		<input type="url" class="regular-text" name="voiceotp_gateway_url" value="<?php echo esc_attr( $value ); ?>" placeholder="https://pbx.example.com:8443" />
		<p class="description"><?php esc_html_e( 'Must use HTTPS. HTTP is rejected for remote gateway access.', 'voiceotp-connector' ); ?></p>
		<?php
	}

	public static function render_username() {
		$value = sanitize_text_field( get_option( 'voiceotp_username', '' ) );
		?>
		<input type="text" class="regular-text" name="voiceotp_username" value="<?php echo esc_attr( $value ); ?>" placeholder="voiceotp_api" />
		<?php
	}

	public static function render_password() {
		$stored = get_option( 'voiceotp_password', '' );
		$masked = $stored ? str_repeat( '●', 12 ) : '';
		?>
		<input type="password" class="regular-text" name="voiceotp_password" value="" placeholder="••••••••" />
		<p class="description">
			<?php
			if ( $stored ) {
				echo esc_html__( 'Password is stored. Leave blank to keep the current password.', 'voiceotp-connector' );
			} else {
				echo esc_html__( 'Set a high-entropy API password matching the gateway configuration.', 'voiceotp-connector' );
			}
			?>
		</p>
		<?php
	}

	public static function render_timeout() {
		$value = absint( get_option( 'voiceotp_timeout', 10 ) );
		?>
		<input type="number" min="1" max="300" step="1" class="small-text" name="voiceotp_timeout" value="<?php echo esc_attr( $value ); ?>" />
		<?php
	}

	public static function render_verify_ssl() {
		$value = get_option( 'voiceotp_verify_ssl', '1' );
		?>
		<input type="checkbox" name="voiceotp_verify_ssl" value="1"<?php checked( $value, '1' ); ?> />
		<span class="description"><?php esc_html_e( 'Disable only for self-signed certificates in a trusted private network.', 'voiceotp-connector' ); ?></span>
		<?php
	}

	/**
	 * Sanitize the enabled flag.
	 */
	public static function sanitize_enabled( $value ) {
		return ( '1' === $value ) ? '1' : '0';
	}

	/**
	 * Sanitize the gateway URL.
	 */
	public static function sanitize_url( $value ) {
		$value = trim( $value );
		if ( '' === $value ) {
			return '';
		}
		if ( strpos( $value, 'https://' ) !== 0 && strpos( $value, 'http://' ) !== 0 ) {
			$value = 'https://' . $value;
		}
		$url = esc_url_raw( $value );
		if ( ! $url || strpos( $url, 'https://') !== 0 ) {
			add_settings_error( 'voiceotp_gateway_url', 'voiceotp_url_insecure', __( 'Gateway URL must use HTTPS.', 'voiceotp-connector' ), 'error' );
			return get_option( 'voiceotp_gateway_url', '' );
		}
		return trailingslashit( $url );
	}

	/**
	 * Sanitize password without overwriting an existing value when empty.
	 */
	public static function sanitize_password( $new_value ) {
		if ( empty( $new_value ) ) {
			return get_option( 'voiceotp_password', '' );
		}
		return $new_value;
	}

}
