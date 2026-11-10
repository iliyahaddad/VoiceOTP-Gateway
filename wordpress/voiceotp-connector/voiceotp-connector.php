<?php
/**
 * VoiceOTP Connector
 *
 * Connects WordPress to the VoiceOTP Gateway for voice-based OTP delivery.
 *
 * @package   VoiceOTP_Connector
 * @author    VoiceOTP Project
 * @license   MIT
 * @link      https://github.com/voiceotp/gateway
 */

namespace VoiceOTP\Connector;

if ( ! defined( 'ABSPATH' ) ) {
	exit;
}

/**
 * Plugin bootstrap.
 */
class Plugin {

	const VERSION = '1.0.0';
	const SLUG    = 'voiceotp-connector';

	private static $instance = null;

	public static function instance() {
		if ( null === self::$instance ) {
			self::$instance = new self();
			self::$instance->init();
		}
		return self::$instance;
	}

	private function init() {
		require_once __DIR__ . '/includes/class-client.php';
		require_once __DIR__ . '/admin/class-settings.php';
		require_once __DIR__ . '/admin/class-test.php';

		add_action( 'admin_init', array( $this, 'register_settings' ) );
		add_action( 'admin_init', array( Test::class, 'register' ) );
		add_action( 'admin_menu', array( $this, 'add_admin_menu' ) );
		add_action( 'admin_enqueue_scripts', array( $this, 'admin_assets' ) );
		add_action( 'plugin_action_links_' . plugin_basename( __FILE__ ), array( $this, 'plugin_action_links' ) );
		register_activation_hook( __FILE__, array( $this, 'activate' ) );
	}

	public function activate() {
		Settings::init_defaults();
	}

	public function register_settings() {
		Settings::register();
	}

	public function add_admin_menu() {
		add_options_page(
			__( 'VoiceOTP Settings', 'voiceotp-connector' ),
			__( 'VoiceOTP', 'voiceotp-connector' ),
			'manage_options',
			self::SLUG,
			array( Settings::class, 'render_page' )
		);
		add_submenu_page(
			'options-general.php?page=' . self::SLUG,
			__( 'Test Connection', 'voiceotp-connector' ),
			__( 'Test Connection', 'voiceotp-connector' ),
			'manage_options',
			self::SLUG . '-test',
			array( Test::class, 'render_form' )
		);
	}

	public function admin_assets( $hook ) {
		if ( $hook !== 'toplevel_page_' . self::SLUG && $hook !== 'settings_page_' . self::SLUG ) {
			return;
		}
		// No frontend JavaScript is used by design.
		wp_enqueue_style( 'voiceotp-connector-admin', false );
	}

	public function plugin_action_links( $links ) {
		$settings_link = '<a href="' . esc_url( admin_url( 'options-general.php?page=' . self::SLUG ) ) . '">' . __( 'Settings', 'voiceotp-connector' ) . '</a>';
		array_unshift( $links, $settings_link );
		return $links;
	}
}

Plugin::instance();
