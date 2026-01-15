fn main() {
    tauri_build::build();

    // Link EventKit framework on macOS (objc2 handles the binding)
    #[cfg(target_os = "macos")]
    {
        println!("cargo:rustc-link-lib=framework=Foundation");
        println!("cargo:rustc-link-lib=framework=EventKit");
        println!("cargo:rustc-link-lib=framework=AppKit");
    }
}
