# WAXE Desktop Application Specifications

## Overview

WAXE Desktop is a cross-platform accessibility testing application built with Tauri that transforms the existing CLI-based Wellington Axe (WAXE) tool into a user-friendly desktop application. The app enables users to perform comprehensive accessibility audits of websites using axe-core, with an intuitive interface for managing projects, viewing results, and generating reports.

## Core Concept

Transform WAXE from a command-line tool into a downloadable desktop application that provides:
- Visual project management for accessibility testing
- Real-time scanning progress and results
- Interactive reporting and data visualization
- Cross-platform compatibility (Windows, macOS, Linux)
- Complete offline functionality

## Technology Stack

### Frontend
- **Framework**: React 18+ with TypeScript
- **UI Library**: Modern component library (Chakra UI, Mantine, or Ant Design)
- **State Management**: Zustand or Redux Toolkit
- **Styling**: CSS-in-JS or Tailwind CSS
- **Charts/Visualization**: Recharts or Chart.js

### Backend
- **Runtime**: Tauri (Rust)
- **Browser Automation**: Playwright (via Node.js subprocess or native Rust implementation)
- **Accessibility Engine**: axe-core via @axe-core/playwright
- **File Operations**: Native Rust std::fs
- **Template Engine**: Handlebars or Tera (Rust)

### Build & Distribution
- **Bundler**: Tauri's built-in bundler
- **Updates**: Tauri Updater for automatic updates
- **Installers**: Native installers for each platform (MSI, DMG, AppImage)

## Architecture

```
src-tauri/                 # Rust backend
├── src/
│   ├── main.rs           # Tauri app entry point
│   ├── commands/         # Tauri command handlers
│   │   ├── scanner.rs    # Accessibility scanning logic
│   │   ├── projects.rs   # Project management
│   │   ├── reports.rs    # Report generation
│   │   └── sitemap.rs    # URL discovery
│   ├── models/           # Data structures
│   └── utils/            # Helper functions
├── Cargo.toml           # Rust dependencies
└── tauri.conf.json      # Tauri configuration

src/                      # React frontend
├── components/
│   ├── Scanner/         # Scanning interface components
│   ├── Reports/         # Report viewer and management
│   ├── Projects/        # Project creation and management
│   ├── Settings/        # Application settings
│   └── shared/          # Reusable UI components
├── stores/              # State management
├── types/               # TypeScript type definitions
├── utils/               # Frontend utilities
├── hooks/               # Custom React hooks
└── assets/              # Static assets

public/                   # Static assets
dist/                     # Built frontend assets
```

## Key Features

### 1. Project Management
- **Create Projects**: Set up new accessibility testing projects with custom configurations
- **Import URL Lists**: Upload text files or CSV files containing URLs to test
- **Sitemap Integration**: Automatically discover URLs from XML sitemaps
- **Site Configurations**: Predefined settings for common sites (Wellington.govt.nz, etc.)
- **Custom Rules**: Configure which axe-core rules to include/exclude per project
- **Project Templates**: Save and reuse common project configurations

### 2. Scanning Interface
- **Real-time Progress**: Visual progress bars with ETA and completion status
- **Concurrent Processing**: Configurable concurrent scan limits
- **Scan Scheduling**: Schedule scans to run at specific times
- **Background Processing**: Continue scans while using other app features
- **Pause/Resume**: Ability to pause and resume long-running scans
- **Scan History**: Track and compare results over time

### 3. Results & Reporting
- **Interactive Dashboard**: Visual overview of scan results with charts and statistics
- **Detailed Violation View**: Expandable list of violations with code snippets and recommendations
- **Filtering & Search**: Filter results by severity, rule type, or affected URLs
- **Report Generation**: Export results as HTML, PDF, or CSV
- **Comparison Mode**: Side-by-side comparison of scan results across different time periods
- **Screenshot Integration**: Capture screenshots of pages with violations

### 4. Desktop Integration
- **System Notifications**: Native notifications for scan completion
- **File Associations**: Associate .waxe project files with the application
- **Drag & Drop**: Drag URL files directly into the application
- **Native Dialogs**: Use system file dialogs for import/export operations
- **Auto-updates**: Automatic application updates via Tauri updater
- **System Tray**: Minimize to system tray for background operation

## User Interface Design

### Main Application Layout
```
┌─────────────────────────────────────────────────────────────┐
│ WAXE                                    [_] [□] [×]          │
├─────────────────────────────────────────────────────────────┤
│ File  Edit  View  Scan  Reports  Help                      │
├─────────────────────────────────────────────────────────────┤
│ [Sidebar]                    [Main Content Area]           │
│ • Projects                   ┌─────────────────────────┐   │
│   - Wellington Site          │                         │   │
│   - Let's Talk              │    Dashboard/Content     │   │
│   - Archives                │                         │   │
│ • Recent Scans              │                         │   │
│ • Settings                  │                         │   │
│                             └─────────────────────────┘   │
├─────────────────────────────────────────────────────────────┤
│ Status: Ready | Last scan: 2 hours ago | 0 active scans   │
└─────────────────────────────────────────────────────────────┘
```

### Key Screens

#### 1. Project Creation Screen
- Project name and description
- URL import options (file upload, manual entry, sitemap)
- Accessibility rule configuration
- Scan settings (concurrency, timeouts, etc.)

#### 2. Scanning Dashboard
- Real-time progress visualization
- Current scan status and logs
- Ability to pause/cancel scans
- Queue management for multiple scans

#### 3. Results Viewer
- Filterable table of violations
- Severity-based color coding
- Expandable violation details
- Quick actions (re-scan URL, mark as false positive)

#### 4. Report Generator
- Template selection for reports
- Custom branding options
- Export format selection
- Scheduled report generation

## Technical Implementation

### Tauri Commands (Rust ↔ JavaScript Bridge)

```rust
// Core scanning functionality
#[tauri::command]
async fn scan_urls(urls: Vec<String>, config: ScanConfig) -> Result<ScanResults, String>

#[tauri::command]  
async fn get_scan_progress(scan_id: String) -> Result<ScanProgress, String>

#[tauri::command]
async fn cancel_scan(scan_id: String) -> Result<(), String>

// Project management
#[tauri::command]
async fn save_project(project: Project) -> Result<String, String>

#[tauri::command]
async fn load_project(project_id: String) -> Result<Project, String>

#[tauri::command]
async fn delete_project(project_id: String) -> Result<(), String>

// Sitemap and URL discovery
#[tauri::command]
async fn generate_sitemap(url: String, max_urls: Option<usize>) -> Result<Vec<String>, String>

// Report generation
#[tauri::command]
async fn generate_report(results: ScanResults, template: ReportTemplate) -> Result<String, String>

#[tauri::command]
async fn export_report(report_path: String, format: ExportFormat) -> Result<(), String>

// File system operations
#[tauri::command]
async fn import_url_list(file_path: String) -> Result<Vec<String>, String>

#[tauri::command]
async fn save_scan_results(results: ScanResults, file_path: String) -> Result<(), String>
```

### Data Models

```typescript
// Frontend TypeScript interfaces
interface Project {
  id: string;
  name: string;
  description?: string;
  urls: string[];
  config: ScanConfig;
  created_at: Date;
  updated_at: Date;
}

interface ScanConfig {
  exclude_rules?: string[];
  include_rules?: string[];
  max_concurrent: number;
  timeout: number;
  viewport: { width: number; height: number };
  user_agent?: string;
}

interface ScanResults {
  id: string;
  project_id: string;
  scan_date: Date;
  total_urls: number;
  processed_urls: number;
  violations: Violation[];
  duration: number;
  status: 'running' | 'completed' | 'failed' | 'cancelled';
}

interface Violation {
  url: string;
  rule_id: string;
  severity: 'minor' | 'moderate' | 'serious' | 'critical';
  description: string;
  help_url: string;
  nodes: ViolationNode[];
}

interface ViolationNode {
  html: string;
  selector: string;
  xpath: string;
  message: string;
}
```

### State Management

```typescript
// Zustand store for application state
interface AppStore {
  // Projects
  projects: Project[];
  activeProject: Project | null;
  setActiveProject: (project: Project) => void;
  createProject: (project: Omit<Project, 'id'>) => void;
  
  // Scans
  activeScans: Map<string, ScanProgress>;
  scanResults: ScanResults[];
  startScan: (projectId: string) => Promise<string>;
  cancelScan: (scanId: string) => Promise<void>;
  
  // UI State
  sidebarOpen: boolean;
  currentView: 'dashboard' | 'projects' | 'results' | 'settings';
  toggleSidebar: () => void;
  setCurrentView: (view: string) => void;
}
```

## Migration Strategy

### Phase 1: Core Functionality
1. Set up Tauri project structure
2. Migrate existing scanning logic to Rust commands
3. Create basic React frontend with project management
4. Implement real-time progress tracking

### Phase 2: Enhanced Features
1. Add interactive results viewer
2. Implement report generation and export
3. Create scheduling and background scan capabilities
4. Add comparison and historical tracking

### Phase 3: Desktop Integration
1. Implement native notifications
2. Add auto-updater functionality
3. Create system tray integration
4. Add file associations and drag-drop support

### Phase 4: Polish & Distribution
1. Comprehensive testing across platforms
2. User documentation and help system
3. Setup CI/CD for automated builds
4. Create distribution packages and installer

## Security Considerations

- **Sandboxing**: Leverage Tauri's security model to limit system access
- **Content Security Policy**: Strict CSP for the frontend webview
- **API Validation**: Validate all inputs at the Rust command layer
- **File Access**: Limit file system access to designated directories
- **Network Requests**: Restrict network access to scanning operations only
- **Data Encryption**: Encrypt sensitive project data at rest

## Performance Optimizations

- **Lazy Loading**: Load scan results and projects on demand
- **Virtualization**: Use virtual scrolling for large result sets
- **Background Processing**: Offload scanning to background threads
- **Caching**: Cache scan results and project data efficiently
- **Batch Operations**: Batch file operations for better performance
- **Memory Management**: Optimize memory usage for large scans

## Distribution & Updates

### Packaging
- **Windows**: MSI installer with automatic shortcuts
- **macOS**: DMG with drag-to-Applications setup
- **Linux**: AppImage, .deb, and .rpm packages

### Auto-Updates
- Implement Tauri's built-in updater
- Check for updates on application startup
- Download and install updates in background
- Notify users of available updates

### Installation Size
- Target ~50-100MB total application size
- Bundle minimal Chromium runtime for Playwright
- Optimize asset compression and tree-shaking

## Future Enhancements

- **Plugin System**: Allow third-party extensions
- **Cloud Sync**: Optional cloud backup for projects and results
- **Team Collaboration**: Share projects and results across teams
- **Custom Rules**: Create and share custom accessibility rules
- **Integration APIs**: Connect with external tools and services
- **Mobile Companion**: Mobile app for viewing results on-the-go

## Success Metrics

- **User Adoption**: Number of downloads and active users
- **Usage Patterns**: Frequency of scans and project creation
- **Performance**: Scan completion times and application responsiveness
- **Stability**: Crash rates and error reporting
- **User Satisfaction**: Feedback and rating scores