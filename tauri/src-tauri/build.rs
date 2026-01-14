fn main() {
    tauri_build::build();

    // Compile Objective-C code and link EventKit on macOS
    #[cfg(target_os = "macos")]
    {
        println!("cargo:rerun-if-changed=src/calendar/eventkit.m");

        cc::Build::new()
            .file("src/calendar/eventkit.m")
            .flag("-fobjc-arc")
            .compile("eventkit");

        println!("cargo:rustc-link-lib=framework=Foundation");
        println!("cargo:rustc-link-lib=framework=EventKit");
        println!("cargo:rustc-link-lib=framework=AppKit");
    }
}
