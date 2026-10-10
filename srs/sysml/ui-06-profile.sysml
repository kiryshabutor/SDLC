// Рисунок А.8 — макет: профиль и настройки (REQ-FUNC-005…REQ-FUNC-007, REQ-FUNC-010)
package UI_06_Profile {
    private import UiKit::*;

    part <profile> 'Профиль' : Screen {
        part 'Панель навигации' : NavBar {
            part 'Вишлист' : Link;
            part 'Поиск' : Link;
            part 'Уведомления (2)' : Link;
            part '[Профиль]' : Link;
        }
        part 'Настройки' : Row {
            part 'Учётная запись' : Form {
                part 'Email: ivan@mail.by ✓ подтверждён' : Label;
                part 'Регион магазина: Беларусь (BY) ▾' : Select;
                part '⚠ При смене региона условия «цена ниже X» сбросятся на «любая скидка»' : Banner;
                part '☑ Отправлять уведомления на email' : Checkbox;
                part 'Сохранить' : Button;
                part 'Выйти' : Button;
            }
            part 'Импорт вишлиста Steam' : Form {
                part 'Ссылка на профиль [steamcommunity.com/id/ivan]' : Input;
                part 'Импортировать' : Button;
                part 'Добавлено 12 игр, пропущено 3 (уже в вишлисте)' : Label;
            }
            part 'Удаление учётной записи' : Form {
                part 'Данные будут удалены безвозвратно' : Label;
                part 'Текущий пароль [••••••••]' : Input;
                part 'Удалить учётную запись' : Button;
            }
        }
        part 'Подвал' : Footer {
            part 'Цены: магазин Steam · история: IsThereAnyDeal · не связано с Valve' : Label;
        }
    }
}
