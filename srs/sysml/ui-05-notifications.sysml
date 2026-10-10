// Рисунок А.7 — макет: центр уведомлений (REQ-FUNC-028)
package UI_05_Notifications {
    private import UiKit::*;

    part <notifications> 'Уведомления' : Screen {
        part 'Панель навигации' : NavBar {
            part 'Вишлист' : Link;
            part 'Поиск' : Link;
            part '[Уведомления (2)]' : Link;
            part 'Профиль' : Link;
        }
        part 'Заголовок' : Row {
            part 'Уведомления за последние 90 дней' : Label;
            part 'Отметить все прочитанными' : Button;
        }
        part 'Список уведомлений (новые сверху)' : ListView {
            part 'Уведомление 1 (не прочитано)' : ListItem {
                part '● 10.10.2026 14:00' : Label;
                part 'Celeste' : Link;
                part '$4.99  −75 %' : Label;
                part '★ Исторический минимум' : Label;
            }
            part 'Уведомление 2 (не прочитано)' : ListItem {
                part '● 10.10.2026 14:00' : Label;
                part 'Hades' : Link;
                part '$12.49  −50 %' : Label;
            }
            part 'Уведомление 3' : ListItem {
                part '08.10.2026 08:00' : Label;
                part 'Stardew Valley' : Link;
                part '$8.99  −40 %' : Label;
            }
        }
        part 'Подвал' : Footer {
            part 'Цены: магазин Steam · история: IsThereAnyDeal · не связано с Valve' : Label;
        }
    }
}
