package ke.co.baharilink.baharimrv.ui

import android.net.Uri
import androidx.browser.customtabs.CustomTabsIntent
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBars
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.material3.Icon
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.NavigationBarItemDefaults
import androidx.compose.material3.NavigationRail
import androidx.compose.material3.NavigationRailItem
import androidx.compose.material3.NavigationRailItemDefaults
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.annotation.DrawableRes
import ke.co.baharilink.baharimrv.R
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavController
import androidx.navigation.NavGraph.Companion.findStartDestination
import androidx.navigation.NavType
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import androidx.navigation.navArgument
import ke.co.baharilink.baharimrv.ui.auth.AuthScreen
import ke.co.baharilink.baharimrv.ui.auth.AuthViewModel
import ke.co.baharilink.baharimrv.ui.components.LoadingState
import ke.co.baharilink.baharimrv.ui.components.OfflineBanner
import ke.co.baharilink.baharimrv.ui.credits.CreditsScreen
import ke.co.baharilink.baharimrv.ui.home.HomeScreen
import ke.co.baharilink.baharimrv.ui.projects.AddActivityScreen
import ke.co.baharilink.baharimrv.ui.projects.AddPlotScreen
import ke.co.baharilink.baharimrv.ui.projects.MonitorManagementScreen
import ke.co.baharilink.baharimrv.ui.projects.ProjectDetailScreen
import ke.co.baharilink.baharimrv.ui.projects.ProjectsScreen
import ke.co.baharilink.baharimrv.ui.monitor.MonitorScreen
import ke.co.baharilink.baharimrv.ui.profile.ProfileScreen
import ke.co.baharilink.baharimrv.ui.verification.VerificationScreen
import ke.co.baharilink.baharimrv.ui.wallet.WalletScreen

private object Routes {
    const val HOME = "home"
    const val PROJECTS = "projects"
    const val MONITOR = "monitor"
    const val CREDITS = "credits"
    const val WALLET = "wallet"
    const val PROFILE = "profile"
    const val VERIFICATION = "verification?activityId={activityId}"
    const val PROJECT_DETAIL = "projectDetail/{projectId}"
    const val ADD_PLOT = "addPlot/{projectId}"
    const val ADD_ACTIVITY = "addActivity/{projectId}"
    const val MANAGE_MONITORS = "manageMonitors/{projectId}/{plotId}"

    fun verification(activityId: String = "") = "verification?activityId=${Uri.encode(activityId)}"
}

private data class Destination(val route: String, val label: String, @DrawableRes val iconRes: Int)
private val destinations = listOf(
    Destination(Routes.HOME, "Home", R.drawable.ic_home),
    Destination(Routes.MONITOR, "Monitor", R.drawable.ic_chart),
    Destination(Routes.CREDITS, "Credits", R.drawable.ic_co2),
    Destination(Routes.WALLET, "Wallet", R.drawable.ic_wallet),
    Destination(Routes.PROFILE, "Profile", R.drawable.ic_user),
)

@Composable
fun AppRoot(
    authCallback: Uri?,
    onAuthCallbackHandled: () -> Unit,
    authViewModel: AuthViewModel = hiltViewModel(),
    shellViewModel: ShellViewModel = hiltViewModel(),
) {
    val context = LocalContext.current
    val session by authViewModel.session.collectAsStateWithLifecycle()
    val online by shellViewModel.online.collectAsStateWithLifecycle()

    // The authorization URL is collected here because this composable outlives both
    // the signed-out and signed-in trees; a link started from Profile has nowhere else to land.
    LaunchedEffect(authViewModel) {
        authViewModel.openAuthorization.collect { uri ->
            CustomTabsIntent.Builder().setShowTitle(true).build().launchUrl(context, uri)
        }
    }
    LaunchedEffect(authCallback) {
        if (authCallback != null) {
            authViewModel.handleCallback(authCallback)
            onAuthCallbackHandled()
        }
    }

    when {
        session == null -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) { LoadingState("Restoring secure session…") }
        session?.isAuthenticated != true -> AuthScreen(authViewModel)
        else -> AuthenticatedShell(online, authViewModel)
    }
}

@Composable
private fun AuthenticatedShell(online: Boolean, authViewModel: AuthViewModel) {
    val navController = rememberNavController()
    val backStack by navController.currentBackStackEntryAsState()
    val currentRoute = backStack?.destination?.route

    BoxWithConstraints(modifier = Modifier.fillMaxSize()) {
        val compact = maxWidth < 600.dp
        Row(Modifier.fillMaxSize()) {
            if (!compact) {
                AppNavigationRail(navController, currentRoute)
            }
            Scaffold(
                modifier = Modifier.weight(1f).fillMaxHeight(),
                contentWindowInsets = WindowInsets(0, 0, 0, 0),
                // Each Conservation screen draws its own compact context header, so the shell
                // reserves the status inset only instead of repeating the screen title.
                topBar = {
                    Column {
                        Spacer(Modifier.windowInsetsPadding(WindowInsets.statusBars))
                        if (!online) OfflineBanner()
                    }
                },
                bottomBar = {
                    if (compact) AppNavigationBar(navController, currentRoute)
                },
            ) { padding ->
                NavHost(
                    navController = navController,
                    startDestination = Routes.HOME,
                    modifier = Modifier.fillMaxSize().padding(padding).imePadding(),
                ) {
                    composable(Routes.HOME) {
                        HomeScreen(
                            onFieldVisit = { navController.navigate(Routes.MONITOR) },
                            onVerification = { navController.navigate(Routes.verification()) },
                            onProjects = { navController.navigate(Routes.PROJECTS) },
                        )
                    }
                    composable(Routes.MONITOR) { MonitorScreen() }
                    composable(Routes.PROJECTS) { backStackEntry -> ProjectsScreen(onProjectClick = { id -> navController.navigate(Routes.PROJECT_DETAIL.replace("{projectId}", id)) }) }
                    composable(
                        route = Routes.PROJECT_DETAIL,
                        arguments = listOf(navArgument("projectId") { type = NavType.StringType }),
                    ) { backStackEntry ->
                        ProjectDetailScreen(
                            projectId = backStackEntry.arguments?.getString("projectId").orEmpty(),
                            onBack = { navController.popBackStack() },
                            onAddPlot = { navController.navigate(Routes.ADD_PLOT.replace("{projectId}", backStackEntry.arguments?.getString("projectId").orEmpty())) },
                            onAddActivity = { navController.navigate(Routes.ADD_ACTIVITY.replace("{projectId}", backStackEntry.arguments?.getString("projectId").orEmpty())) },
                            onManageMonitors = { navController.navigate(Routes.MANAGE_MONITORS.replace("{projectId}", backStackEntry.arguments?.getString("projectId").orEmpty()).replace("{plotId}", "")) },
                        )
                    }
                    composable(
                        route = Routes.ADD_PLOT,
                        arguments = listOf(navArgument("projectId") { type = NavType.StringType }),
                    ) { backStackEntry ->
                        AddPlotScreen(
                            projectId = backStackEntry.arguments?.getString("projectId").orEmpty(),
                            onBack = { navController.popBackStack() },
                            onSaved = { navController.popBackStack() },
                        )
                    }
                    composable(
                        route = Routes.ADD_ACTIVITY,
                        arguments = listOf(navArgument("projectId") { type = NavType.StringType }),
                    ) { backStackEntry ->
                        AddActivityScreen(
                            projectId = backStackEntry.arguments?.getString("projectId").orEmpty(),
                            onBack = { navController.popBackStack() },
                        )
                    }
                    composable(Routes.CREDITS) { CreditsScreen() }
                    composable(Routes.WALLET) { WalletScreen() }
                    composable(Routes.PROFILE) { ProfileScreen(authViewModel) }
                    composable(
                        route = Routes.VERIFICATION,
                        arguments = listOf(navArgument("activityId") {
                            type = NavType.StringType
                            defaultValue = ""
                        }),
                    ) { VerificationScreen(onBack = { navController.popBackStack() }) }
                }
            }
        }
    }
}

@Composable
private fun AppNavigationBar(navController: NavController, currentRoute: String?) {
    Column {
        HorizontalDivider(color = MaterialTheme.colorScheme.outline, thickness = 1.dp)
        NavigationBar(
            containerColor = MaterialTheme.colorScheme.surface,
            tonalElevation = 0.dp,
            modifier = Modifier.height(54.dp)
        ) {
            destinations.forEach { destination ->
                val isSelected = currentRoute == destination.route
                val iconColor = if (isSelected) {
                    MaterialTheme.colorScheme.primary
                } else {
                    Color(0xFF9AA8BA)
                }
                
                NavigationBarItem(
                    selected = isSelected,
                    onClick = { navigateTo(navController, destination.route) },
                    icon = {
                        Icon(
                            painter = painterResource(id = destination.iconRes),
                            contentDescription = destination.label,
                            tint = iconColor
                        )
                    },
                    label = {
                        Text(
                            text = destination.label,
                            fontSize = 8.5.sp,
                            fontWeight = androidx.compose.ui.text.font.FontWeight.SemiBold,
                            color = iconColor
                        )
                    },
                    colors = NavigationBarItemDefaults.colors(
                        selectedIconColor = MaterialTheme.colorScheme.primary,
                        selectedTextColor = MaterialTheme.colorScheme.primary,
                        indicatorColor = Color.Transparent,
                        unselectedIconColor = Color(0xFF9AA8BA),
                        unselectedTextColor = Color(0xFF9AA8BA),
                    ),
                )
            }
        }
    }
}

@Composable
private fun AppNavigationRail(navController: NavController, currentRoute: String?) {
    // The wireframe bar is flat white with a green selection; M3's default lavender container
    // and blue pill read as another product's brand, so both surfaces override them here.
    NavigationRail(
        modifier = Modifier.fillMaxHeight(),
        containerColor = MaterialTheme.colorScheme.surface,
    ) {
        destinations.forEach { destination ->
            val isSelected = currentRoute == destination.route
            NavigationRailItem(
                selected = isSelected,
                onClick = { navigateTo(navController, destination.route) },
                icon = {
                    Icon(
                        painter = painterResource(id = destination.iconRes),
                        contentDescription = destination.label
                    )
                },
                label = { Text(destination.label) },
                colors = NavigationRailItemDefaults.colors(
                    selectedIconColor = MaterialTheme.colorScheme.primary,
                    selectedTextColor = MaterialTheme.colorScheme.primary,
                    indicatorColor = Color.Transparent,
                    unselectedIconColor = MaterialTheme.colorScheme.onSurfaceVariant,
                    unselectedTextColor = MaterialTheme.colorScheme.onSurfaceVariant,
                ),
            )
        }
    }
}

private fun navigateTo(navController: NavController, route: String) {
    navController.navigate(route) {
        popUpTo(navController.graph.findStartDestination().id) { saveState = true }
        launchSingleTop = true
        restoreState = true
    }
}
