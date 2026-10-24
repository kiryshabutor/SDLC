// Рисунок А.4 — макет: вишлист, главный экран (REQ-FUNC-011…REQ-FUNC-016, REQ-FUNC-020)
package UI_02_Wishlist {
    private import UiKit::*;

    part <wishlist> 'Вишлист' : Screen {
        part 'Панель навигации' : NavBar {
            part '[Вишлист]' : Link;
            part 'Поиск' : Link;
            part 'Уведомления (2)' : Link;
            part 'Профиль' : Link;
        }
        part '⚠ Email не подтверждён — уведомления на почту не отправляются' : Banner {
            part 'Отправить письмо повторно' : Button;
        }
        part 'Заголовок' : Row {
            part 'Мой вишлист · 37 игр · регион BY' : Label;
            part 'Проверено: 10.10.2026 14:00' : Label;
            part '⟳ Проверить цены сейчас' : Button;
            part '+ Добавить игру' : Link;
        }
        part 'Сортировка и фильтры' : Row {
            part 'Сортировка: по скидке ▾' : Select;
            part '☐ Только со скидкой' : Checkbox;
            part '☐ Только на историческом минимуме' : Checkbox;
        }
        part 'Список игр' : ListView {
            part 'Запись 1' : ListItem {
                part '[обложка]' : Image;
                part 'Hades' : Link;
                part '$12.49  −50 %  (обычно $24.99)' : Label;
                part 'Мин. $9.99' : Label;
                part 'Условие ▾ скидка ≥ 50 %' : Select;
                part '✕' : Button;
            }
            part 'Запись 2' : ListItem {
                part '[обложка]' : Image;
                part 'Celeste' : Link;
                part '$4.99  −75 %  (обычно $19.99)' : Label;
                part '★ Исторический минимум сейчас' : Label;
                part 'Условие ▾ исторический минимум' : Select;
                part '✕' : Button;
            }
            part 'Запись 3' : ListItem {
                part '[обложка]' : Image;
                part 'Hollow Knight: Silksong' : Link;
                part 'Цена не объявлена' : Label;
                part 'Условие ▾ любая скидка' : Select;
                part '✕' : Button;
            }
            part 'Запись 4' : ListItem {
                part '[обложка]' : Image;
                part 'Some Game' : Link;
                part 'Недоступна в вашем регионе' : Label;
                part 'Условие ▾ цена ниже $10' : Select;
                part '✕' : Button;
            }
        }
        part 'Подвал' : Footer {
            part 'Цены: магазин Steam · история: IsThereAnyDeal · не связано с Valve' : Label;
        }
    }
}
